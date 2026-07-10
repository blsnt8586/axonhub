package gql

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCommercialGraphQLUserQueriesAreTenantIsolated(t *testing.T) {
	mutationResolver, _, setupCtx, client, _, _ := setupBillingResolversTest(t, "commercial_graphql_tenant_isolation")
	user := createBillingResolverUser(t, setupCtx, client, false)
	other := createBillingResolverUser(t, setupCtx, client, false)

	userAccount, err := mutationResolver.billingAccountService.GetOrCreateForSubject(setupCtx, biz.UserBillingSubject(user.ID))
	require.NoError(t, err)
	otherAccount, err := mutationResolver.billingAccountService.GetOrCreateForSubject(setupCtx, biz.UserBillingSubject(other.ID))
	require.NoError(t, err)
	_, err = mutationResolver.paymentService.AdjustUserBalance(setupCtx, adjustUserBalanceInput(user.ID, "5", "tenant-user-credit"))
	require.NoError(t, err)
	_, err = mutationResolver.paymentService.AdjustUserBalance(setupCtx, adjustUserBalanceInput(other.ID, "9", "tenant-other-credit"))
	require.NoError(t, err)

	serveGraphQL := commercialGraphQLTestHandler(client, mutationResolver.Resolver, user)
	allowed := postCommercialGraphQL(t, serveGraphQL, `
		query MyCommercialData {
			myBillingAccount { ownerID balanceMicros }
			myLedgerTransactions(first: 10) {
				totalCount
				edges { node { billingAccountID amountMicros } }
			}
		}
	`, nil)
	require.Empty(t, allowed.Errors, allowed.Raw)
	accountData := allowed.Data["myBillingAccount"].(map[string]any)
	require.EqualValues(t, user.ID, accountData["ownerID"])
	require.EqualValues(t, 5_000_000, accountData["balanceMicros"])
	ledgerData := allowed.Data["myLedgerTransactions"].(map[string]any)
	require.EqualValues(t, 1, ledgerData["totalCount"])
	edges := ledgerData["edges"].([]any)
	require.Len(t, edges, 1)
	ledgerNode := edges[0].(map[string]any)["node"].(map[string]any)
	require.Equal(t, fmt.Sprintf("gid://axonhub/%s/%d", ent.TypeBillingAccount, userAccount.ID), ledgerNode["billingAccountID"])
	require.NotEqual(t, fmt.Sprintf("gid://axonhub/%s/%d", ent.TypeBillingAccount, otherAccount.ID), ledgerNode["billingAccountID"])

	forbiddenCases := []struct {
		name      string
		query     string
		variables map[string]any
	}{
		{
			name:  "other user wallet",
			query: `query OtherWallet($userId: ID!) { userBillingAccount(userId: $userId) { id balanceMicros } }`,
			variables: map[string]any{
				"userId": fmt.Sprintf("gid://axonhub/%s/%d", ent.TypeUser, other.ID),
			},
		},
		{
			name:  "admin ledger",
			query: `query AdminLedger { adminLedgerTransactions(first: 10) { totalCount } }`,
		},
		{
			name:  "generated billing collection",
			query: `query RawBillingAccounts { billingAccounts(first: 10) { totalCount } }`,
		},
	}
	for _, tc := range forbiddenCases {
		t.Run(tc.name, func(t *testing.T) {
			payload := postCommercialGraphQL(t, serveGraphQL, tc.query, tc.variables)
			require.NotEmpty(t, payload.Errors, payload.Raw)
		})
	}
}

func TestCommercialResolversRejectAllAdminQueriesForNonOwner(t *testing.T) {
	_, queryResolver, setupCtx, client, _, project := setupBillingResolversTest(t, "commercial_graphql_admin_matrix")
	user := createBillingResolverUser(t, setupCtx, client, false)
	other := createBillingResolverUser(t, setupCtx, client, false)
	userCtx := contexts.WithUser(setupCtx, user)
	first := 10

	tests := []struct {
		name string
		call func() error
	}{
		{name: "project billing account", call: func() error {
			_, err := queryResolver.ProjectBillingAccount(userCtx, objects.GUID{Type: ent.TypeProject, ID: project.ID})
			return err
		}},
		{name: "user billing account", call: func() error {
			_, err := queryResolver.UserBillingAccount(userCtx, objects.GUID{Type: ent.TypeUser, ID: other.ID})
			return err
		}},
		{name: "user payment orders", call: func() error {
			_, err := queryResolver.UserPaymentOrders(userCtx, objects.GUID{Type: ent.TypeUser, ID: other.ID}, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "user usage billing records", call: func() error {
			_, err := queryResolver.UserUsageBillingRecords(userCtx, objects.GUID{Type: ent.TypeUser, ID: other.ID}, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "user ledger transactions", call: func() error {
			_, err := queryResolver.UserLedgerTransactions(userCtx, objects.GUID{Type: ent.TypeUser, ID: other.ID}, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin ledger transactions", call: func() error {
			_, err := queryResolver.AdminLedgerTransactions(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin usage billing records", call: func() error {
			_, err := queryResolver.AdminUsageBillingRecords(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin billing holds", call: func() error {
			_, err := queryResolver.AdminBillingHolds(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin payment orders", call: func() error {
			_, err := queryResolver.AdminPaymentOrders(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin payment events", call: func() error {
			_, err := queryResolver.AdminPaymentEvents(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin redeem codes", call: func() error {
			_, err := queryResolver.AdminRedeemCodes(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin promo codes", call: func() error {
			_, err := queryResolver.AdminPromoCodes(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin promo usages", call: func() error {
			_, err := queryResolver.AdminPromoUsages(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin affiliate setting", call: func() error {
			_, err := queryResolver.AdminAffiliateSetting(userCtx)
			return err
		}},
		{name: "admin affiliate profiles", call: func() error {
			_, err := queryResolver.AdminAffiliateProfiles(userCtx, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin affiliate invitations", call: func() error {
			_, err := queryResolver.AdminAffiliateInvitations(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin affiliate rebates", call: func() error {
			_, err := queryResolver.AdminAffiliateRebates(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin notification setting", call: func() error {
			_, err := queryResolver.AdminBillingNotificationSetting(userCtx)
			return err
		}},
		{name: "admin notifications", call: func() error {
			_, err := queryResolver.AdminBillingNotifications(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin commercial setting", call: func() error {
			_, err := queryResolver.AdminCommercialSetting(userCtx)
			return err
		}},
		{name: "admin audit logs", call: func() error {
			_, err := queryResolver.AdminBillingAuditLogs(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin user subscriptions", call: func() error {
			_, err := queryResolver.AdminUserSubscriptions(userCtx, nil, nil, &first, nil, nil, nil)
			return err
		}},
		{name: "admin billing report", call: func() error {
			_, err := queryResolver.AdminBillingReport(userCtx, nil)
			return err
		}},
		{name: "admin billing csv", call: func() error {
			_, err := queryResolver.ExportAdminBillingCSV(userCtx, ExportAdminBillingCSVInput{Dataset: string(biz.BillingCSVExportDatasetLedgerTransactions)})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorIs(t, test.call(), ErrNotOwner)
		})
	}
}

func TestCommercialResolversRejectAllAdminMutationsForNonOwner(t *testing.T) {
	mutationResolver, _, setupCtx, client, _, _ := setupBillingResolversTest(t, "commercial_graphql_admin_mutation_matrix")
	user := createBillingResolverUser(t, setupCtx, client, false)
	userCtx := contexts.WithUser(setupCtx, user)

	tests := []struct {
		name string
		call func() error
	}{
		{name: "create manual recharge", call: func() error {
			_, err := mutationResolver.CreateManualRechargeOrder(userCtx, biz.CreateManualRechargeOrderInput{})
			return err
		}},
		{name: "confirm manual payment", call: func() error {
			_, err := mutationResolver.ConfirmManualPayment(userCtx, biz.ConfirmManualPaymentInput{})
			return err
		}},
		{name: "cancel payment", call: func() error {
			_, err := mutationResolver.CancelPaymentOrder(userCtx, biz.CancelPaymentOrderInput{})
			return err
		}},
		{name: "make up payment", call: func() error {
			_, err := mutationResolver.MakeUpPaymentOrder(userCtx, biz.MakeUpPaymentOrderInput{})
			return err
		}},
		{name: "simulated checkout", call: func() error {
			_, err := mutationResolver.CreateSimulatedEPayRechargeCheckout(userCtx, CreateSimulatedEPayRechargeCheckoutInput{})
			return err
		}},
		{name: "upsert payment provider", call: func() error {
			_, err := mutationResolver.UpsertEPayPaymentProvider(userCtx, UpsertEPayPaymentProviderInput{})
			return err
		}},
		{name: "adjust user balance", call: func() error {
			_, err := mutationResolver.AdjustUserBalance(userCtx, biz.AdjustUserBalanceInput{})
			return err
		}},
		{name: "update user account", call: func() error {
			_, err := mutationResolver.UpdateUserBillingAccount(userCtx, biz.UpdateUserBillingAccountInput{})
			return err
		}},
		{name: "create redeem codes", call: func() error {
			_, err := mutationResolver.CreateRedeemCodes(userCtx, biz.CreateRedeemCodesInput{})
			return err
		}},
		{name: "admin create and redeem", call: func() error {
			_, err := mutationResolver.AdminCreateAndRedeemCode(userCtx, biz.AdminCreateAndRedeemCodeInput{})
			return err
		}},
		{name: "update redeem code", call: func() error {
			_, err := mutationResolver.UpdateRedeemCodeStatus(userCtx, biz.UpdateRedeemCodeStatusInput{})
			return err
		}},
		{name: "delete redeem code", call: func() error {
			_, err := mutationResolver.DeleteRedeemCode(userCtx, objects.GUID{Type: ent.TypeRedeemCode, ID: 1})
			return err
		}},
		{name: "save promo code", call: func() error {
			_, err := mutationResolver.SavePromoCode(userCtx, biz.SavePromoCodeInput{})
			return err
		}},
		{name: "update promo code", call: func() error {
			_, err := mutationResolver.UpdatePromoCodeStatus(userCtx, biz.UpdatePromoCodeStatusInput{})
			return err
		}},
		{name: "delete promo code", call: func() error {
			_, err := mutationResolver.DeletePromoCode(userCtx, objects.GUID{Type: ent.TypePromoCode, ID: 1})
			return err
		}},
		{name: "save subscription plan", call: func() error {
			_, err := mutationResolver.SaveSubscriptionPlan(userCtx, biz.SaveSubscriptionPlanInput{})
			return err
		}},
		{name: "delete subscription plan", call: func() error {
			_, err := mutationResolver.DeleteSubscriptionPlan(userCtx, objects.GUID{Type: ent.TypeSubscriptionPlan, ID: 1})
			return err
		}},
		{name: "assign subscription", call: func() error {
			_, err := mutationResolver.AdminAssignSubscription(userCtx, biz.AdminAssignSubscriptionInput{})
			return err
		}},
		{name: "extend subscription", call: func() error {
			_, err := mutationResolver.ExtendUserSubscription(userCtx, biz.ExtendUserSubscriptionInput{})
			return err
		}},
		{name: "revoke subscription", call: func() error {
			_, err := mutationResolver.RevokeUserSubscription(userCtx, objects.GUID{Type: ent.TypeUserSubscription, ID: 1}, nil)
			return err
		}},
		{name: "restore subscription", call: func() error {
			_, err := mutationResolver.RestoreUserSubscription(userCtx, objects.GUID{Type: ent.TypeUserSubscription, ID: 1})
			return err
		}},
		{name: "reset subscription", call: func() error {
			_, err := mutationResolver.ResetUserSubscriptionUsage(userCtx, objects.GUID{Type: ent.TypeUserSubscription, ID: 1})
			return err
		}},
		{name: "save affiliate setting", call: func() error {
			_, err := mutationResolver.SaveAffiliateSetting(userCtx, biz.SaveAffiliateSettingInput{})
			return err
		}},
		{name: "save affiliate profile", call: func() error {
			_, err := mutationResolver.SaveAffiliateProfile(userCtx, biz.SaveAffiliateProfileInput{})
			return err
		}},
		{name: "save notification setting", call: func() error {
			_, err := mutationResolver.SaveBillingNotificationSetting(userCtx, biz.SaveBillingNotificationSettingInput{})
			return err
		}},
		{name: "save commercial setting", call: func() error {
			_, err := mutationResolver.SaveCommercialSetting(userCtx, biz.SaveCommercialSettingInput{})
			return err
		}},
		{name: "run commercial maintenance", call: func() error {
			_, err := mutationResolver.RunCommercialMaintenance(userCtx, RunCommercialMaintenanceInput{})
			return err
		}},
		{name: "release billing hold", call: func() error {
			_, err := mutationResolver.ReleaseBillingHold(userCtx, objects.GUID{Type: ent.TypeBillingHold, ID: 1}, "unauthorized")
			return err
		}},
		{name: "save billing price", call: func() error {
			_, err := mutationResolver.SaveBillingPriceRule(userCtx, SaveBillingPriceRuleForm{})
			return err
		}},
		{name: "delete billing price", call: func() error {
			_, err := mutationResolver.DeleteBillingPriceRule(userCtx, objects.GUID{Type: ent.TypeBillingPriceRule, ID: 1})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.ErrorIs(t, test.call(), ErrNotOwner)
		})
	}
}

type commercialGraphQLPayload struct {
	Data   map[string]any   `json:"data"`
	Errors []map[string]any `json:"errors"`
	Raw    string           `json:"-"`
}

func commercialGraphQLTestHandler(client *ent.Client, resolver *Resolver, user *ent.User) http.Handler {
	gqlServer := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: resolver}))
	gqlServer.Use(entgql.Transactioner{TxOpener: client})
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		ctx := contexts.WithUser(request.Context(), user)
		gqlServer.ServeHTTP(w, request.WithContext(ctx))
	})
}

func postCommercialGraphQL(t *testing.T, graphQLHandler http.Handler, query string, variables map[string]any) commercialGraphQLPayload {
	t.Helper()
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	graphQLHandler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)

	payload := commercialGraphQLPayload{Raw: recorder.Body.String()}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload
}

func adjustUserBalanceInput(userID int, amount, memo string) biz.AdjustUserBalanceInput {
	return biz.AdjustUserBalanceInput{
		UserID:         userID,
		Direction:      ledgertransaction.DirectionCredit,
		Amount:         decimal.RequireFromString(amount),
		Memo:           memo,
		IdempotencyKey: memo,
	}
}
