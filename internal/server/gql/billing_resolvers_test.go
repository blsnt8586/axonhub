package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"entgo.io/contrib/entgql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billingauditlog"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/commercialsetting"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	entproject "github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	entuser "github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestBillingResolversManualRechargeFlow(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, owner, project := setupBillingResolversTest(t, "billing_resolver_flow")
	defer client.Close()

	ctx = contexts.WithUser(ctx, owner)

	account, err := queryResolver.ProjectBillingAccount(ctx, objects.GUID{Type: ent.TypeProject, ID: project.ID})
	require.NoError(t, err)
	require.Equal(t, int64(0), account.BalanceMicros)

	order, err := mutationResolver.CreateManualRechargeOrder(ctx, biz.CreateManualRechargeOrderInput{
		ProjectID: project.ID,
		Amount:    decimal.RequireFromString("12.34"),
	})
	require.NoError(t, err)
	require.Equal(t, project.ID, order.ProjectID)
	require.Equal(t, int64(12_340_000), order.AmountMicros)

	paid, err := mutationResolver.ConfirmManualPayment(ctx, biz.ConfirmManualPaymentInput{
		OrderNo:         order.OrderNo,
		EventKey:        "manual-event-1",
		ExternalTradeNo: "offline-transfer-1",
	})
	require.NoError(t, err)
	require.NotNil(t, paid.LedgerTransactionID)

	account, err = queryResolver.ProjectBillingAccount(ctx, objects.GUID{Type: ent.TypeProject, ID: project.ID})
	require.NoError(t, err)
	require.Equal(t, int64(12_340_000), account.BalanceMicros)

	ledgerTx, err := client.LedgerTransaction.Get(ctx, *paid.LedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, fmt.Sprint(owner.ID), ledgerTx.CreatedByID)
}

func TestBillingResolversRequireOwner(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, _, project := setupBillingResolversTest(t, "billing_resolver_auth")
	defer client.Close()

	normalUser := createBillingResolverUser(t, ctx, client, false)
	ctx = contexts.WithUser(ctx, normalUser)

	_, err := queryResolver.ProjectBillingAccount(ctx, objects.GUID{Type: ent.TypeProject, ID: project.ID})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.CreateManualRechargeOrder(ctx, biz.CreateManualRechargeOrderInput{
		ProjectID: project.ID,
		Amount:    decimal.NewFromInt(1),
	})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.ConfirmManualPayment(ctx, biz.ConfirmManualPaymentInput{OrderNo: "pay_missing"})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.UpsertEPayPaymentProvider(ctx, UpsertEPayPaymentProviderInput{
		Name:       "Prod ePay",
		GatewayURL: "https://pay.example.com/submit.php",
		Pid:        "1002",
		Key:        ptr("secret-key"),
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/admin/billing",
	})
	require.True(t, errors.Is(err, ErrNotOwner))
}

func TestBillingResolversUpsertEPayPaymentProvider(t *testing.T) {
	mutationResolver, _, ctx, _, owner, _ := setupBillingResolversTest(t, "billing_resolver_epay_provider")

	ctx = contexts.WithUser(ctx, owner)
	status := paymentproviderinstance.StatusEnabled
	provider, err := mutationResolver.UpsertEPayPaymentProvider(ctx, UpsertEPayPaymentProviderInput{
		Name:       "Prod ePay",
		Status:     &status,
		Currency:   ptr("CNY"),
		GatewayURL: "https://pay.example.com/submit.php",
		Pid:        "1002",
		Key:        ptr("secret-key"),
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/admin/billing",
		Type:       ptr("wxpay"),
		SiteName:   ptr("AxonHub Prod"),
	})
	require.NoError(t, err)
	require.Equal(t, "Prod ePay", provider.Name)
	require.Equal(t, paymentproviderinstance.ProviderTypeEpay, provider.ProviderType)
	require.NotContains(t, fmt.Sprint(provider), "secret-key")

	var cfg map[string]string
	require.NoError(t, json.Unmarshal(provider.Config, &cfg))
	require.NotEqual(t, "secret-key", cfg["key"])
	require.NotContains(t, string(provider.Config), "secret-key")

	parsed, err := biz.ParseEPayConfig(provider.Config)
	require.NoError(t, err)
	require.Equal(t, "secret-key", parsed.Key)
}

func TestBillingResolversUserCanCreateMyEPayRechargeCheckout(t *testing.T) {
	mutationResolver, _, ctx, client, _, project := setupBillingResolversTest(t, "billing_resolver_my_checkout")
	user := createBillingResolverUser(t, ctx, client, false)

	provider, err := mutationResolver.paymentService.GetOrCreateSimulatedEPayProvider(ctx, "https://axon.example.com")
	require.NoError(t, err)

	ctx = contexts.WithUser(ctx, user)
	checkout, err := mutationResolver.CreateMyEPayRechargeCheckout(ctx, CreateMyEPayRechargeCheckoutInput{
		ProjectID:          &objects.GUID{Type: ent.TypeProject, ID: project.ID},
		Amount:             decimal.RequireFromString("20.00"),
		Currency:           ptr("CNY"),
		Subject:            ptr("user recharge"),
		ProviderInstanceID: &objects.GUID{Type: ent.TypePaymentProviderInstance, ID: provider.ID},
	})
	require.NoError(t, err)
	require.Equal(t, paymentproviderinstance.ProviderTypeEpay.String(), checkout.ProviderType)
	require.Equal(t, "redirect", checkout.Method)
	require.NotNil(t, checkout.URL)
	require.Contains(t, *checkout.URL, "/payment/simulate/epay/submit?")
	require.True(t, checkout.Amount.Equal(decimal.RequireFromString("20")))

	order, err := client.PaymentOrder.Query().
		Where(paymentorder.OrderNoEQ(checkout.OrderNo)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, project.ID, order.ProjectID)

	account, err := client.BillingAccount.Get(ctx, order.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, billingaccount.OwnerTypeUser, account.OwnerType)
	require.Equal(t, user.ID, account.OwnerID)
}

func TestBillingResolversRejectsMyEPayCheckoutWithoutUser(t *testing.T) {
	mutationResolver, _, ctx, _, _, _ := setupBillingResolversTest(t, "billing_resolver_my_checkout_no_user")

	_, err := mutationResolver.CreateMyEPayRechargeCheckout(ctx, CreateMyEPayRechargeCheckoutInput{
		Amount: decimal.RequireFromString("20.00"),
	})
	require.True(t, errors.Is(err, ErrNotOwner))
}

func TestBillingResolversUserBillingQueriesAreScopedToCurrentUser(t *testing.T) {
	_, queryResolver, ctx, client, _, project := setupBillingResolversTest(t, "billing_resolver_my_billing_scope")
	user := createBillingResolverUser(t, ctx, client, false)
	other := createBillingResolverUser(t, ctx, client, false)

	userAccount, err := queryResolver.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(user.ID))
	require.NoError(t, err)
	otherAccount, err := queryResolver.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(other.ID))
	require.NoError(t, err)

	ledgerSvc := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, userAccount.ID, decimal.RequireFromString("5"), ledgertransaction.TypePaymentRecharge, "user-credit")
	require.NoError(t, err)
	_, err = ledgerSvc.Credit(ctx, otherAccount.ID, decimal.RequireFromString("9"), ledgertransaction.TypePaymentRecharge, "other-credit")
	require.NoError(t, err)

	_, err = client.PaymentOrder.Create().
		SetOrderNo("pay_user_scope").
		SetProjectID(project.ID).
		SetBillingAccountID(userAccount.ID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(5_000_000).
		SetCurrency("CNY").
		SetStatus(paymentorder.StatusPaid).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentOrder.Create().
		SetOrderNo("pay_other_scope").
		SetProjectID(project.ID).
		SetBillingAccountID(otherAccount.ID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(9_000_000).
		SetCurrency("CNY").
		SetStatus(paymentorder.StatusPaid).
		Save(ctx)
	require.NoError(t, err)

	userUsageLog := createBillingResolverUsageLog(t, ctx, client, project.ID, "gpt-test")
	otherUsageLog := createBillingResolverUsageLog(t, ctx, client, project.ID, "gpt-test")

	_, err = client.UsageBillingRecord.Create().
		SetUsageLogID(userUsageLog.ID).
		SetBillingAccountID(userAccount.ID).
		SetProjectID(project.ID).
		SetUserID(user.ID).
		SetModelID("gpt-test").
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("price-user").
		SetChargeAmountMicros(1_000_000).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusCharged).
		SetIdempotencyKey("usage:user").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageBillingRecord.Create().
		SetUsageLogID(otherUsageLog.ID).
		SetBillingAccountID(otherAccount.ID).
		SetProjectID(project.ID).
		SetUserID(other.ID).
		SetModelID("gpt-test").
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("price-other").
		SetChargeAmountMicros(2_000_000).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusCharged).
		SetIdempotencyKey("usage:other").
		Save(ctx)
	require.NoError(t, err)

	userCtx := contexts.WithUser(ctx, user)
	account, err := queryResolver.MyBillingAccount(userCtx)
	require.NoError(t, err)
	require.Equal(t, userAccount.ID, account.ID)
	require.Equal(t, int64(5_000_000), account.BalanceMicros)

	first := 10
	orders, err := queryResolver.MyPaymentOrders(userCtx, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, orders.TotalCount)
	require.Equal(t, userAccount.ID, orders.Edges[0].Node.BillingAccountID)

	usageRecords, err := queryResolver.MyUsageBillingRecords(userCtx, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, usageRecords.TotalCount)
	require.Equal(t, user.ID, usageRecords.Edges[0].Node.UserID)

	ledgerTxs, err := queryResolver.MyLedgerTransactions(userCtx, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerTxs.TotalCount)
	require.Equal(t, userAccount.ID, ledgerTxs.Edges[0].Node.BillingAccountID)
}

func TestBillingResolversOwnerCanAdjustAndQueryUserBalance(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_resolver_owner_adjust_user")
	user := createBillingResolverUser(t, ctx, client, false)

	ownerCtx := contexts.WithUser(ctx, owner)
	tx, err := mutationResolver.AdjustUserBalance(ownerCtx, biz.AdjustUserBalanceInput{
		UserID:         user.ID,
		Direction:      ledgertransaction.DirectionCredit,
		Amount:         decimal.RequireFromString("12.5"),
		IdempotencyKey: "admin-adjust-user-1",
		Memo:           "manual top-up",
	})
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.TypeAdminAdjustment, tx.Type)
	require.Equal(t, fmt.Sprint(owner.ID), tx.CreatedByID)

	account, err := queryResolver.UserBillingAccount(ownerCtx, objects.GUID{Type: ent.TypeUser, ID: user.ID})
	require.NoError(t, err)
	require.Equal(t, billingaccount.OwnerTypeUser, account.OwnerType)
	require.Equal(t, user.ID, account.OwnerID)
	require.Equal(t, int64(12_500_000), account.BalanceMicros)

	first := 10
	ledgerTxs, err := queryResolver.UserLedgerTransactions(ownerCtx, objects.GUID{Type: ent.TypeUser, ID: user.ID}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerTxs.TotalCount)
	require.Equal(t, tx.ID, ledgerTxs.Edges[0].Node.ID)
}

func TestBillingResolversOwnerCanQueryAdminBillingOperationsWithFilters(t *testing.T) {
	_, queryResolver, ctx, client, owner, project := setupBillingResolversTest(t, "billing_resolver_admin_operation_filters")
	user := createBillingResolverUser(t, ctx, client, false)
	other := createBillingResolverUser(t, ctx, client, false)

	userAccount, err := queryResolver.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(user.ID))
	require.NoError(t, err)
	otherAccount, err := queryResolver.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(other.ID))
	require.NoError(t, err)

	ledgerSvc := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	userLedger, err := ledgerSvc.Credit(ctx, userAccount.ID, decimal.RequireFromString("5"), ledgertransaction.TypePaymentRecharge, "admin-filter-user-credit")
	require.NoError(t, err)
	_, err = ledgerSvc.Credit(ctx, otherAccount.ID, decimal.RequireFromString("9"), ledgertransaction.TypePaymentRecharge, "admin-filter-other-credit")
	require.NoError(t, err)

	userOrder, err := client.PaymentOrder.Create().
		SetOrderNo("pay_admin_filter_user").
		SetProjectID(project.ID).
		SetBillingAccountID(userAccount.ID).
		SetProviderType(paymentorder.ProviderTypeEpay).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(5_000_000).
		SetCurrency("CNY").
		SetStatus(paymentorder.StatusPaid).
		SetExternalTradeNo("trade_admin_filter_user").
		Save(ctx)
	require.NoError(t, err)
	otherOrder, err := client.PaymentOrder.Create().
		SetOrderNo("pay_admin_filter_other").
		SetProjectID(project.ID).
		SetBillingAccountID(otherAccount.ID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(9_000_000).
		SetCurrency("CNY").
		SetStatus(paymentorder.StatusPending).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.PaymentEvent.Create().
		SetEventKey("evt_admin_filter_user").
		SetPaymentOrderID(userOrder.ID).
		SetProviderType(paymentevent.ProviderTypeEpay).
		SetEventType("notify").
		SetPayload(objects.JSONRawMessage([]byte(`{"trade_no":"trade_admin_filter_user"}`))).
		SetStatus(paymentevent.StatusProcessed).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.PaymentEvent.Create().
		SetEventKey("evt_admin_filter_other").
		SetPaymentOrderID(otherOrder.ID).
		SetProviderType(paymentevent.ProviderTypeManual).
		SetEventType("manual").
		SetStatus(paymentevent.StatusReceived).
		Save(ctx)
	require.NoError(t, err)

	userUsageLog := createBillingResolverUsageLog(t, ctx, client, project.ID, "gpt-admin-filter")
	otherUsageLog := createBillingResolverUsageLog(t, ctx, client, project.ID, "gpt-admin-filter-other")
	_, err = client.UsageBillingRecord.Create().
		SetUsageLogID(userUsageLog.ID).
		SetBillingAccountID(userAccount.ID).
		SetProjectID(project.ID).
		SetUserID(user.ID).
		SetModelID("gpt-admin-filter").
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("price-user").
		SetChargeAmountMicros(1_000_000).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusCharged).
		SetIdempotencyKey("usage:admin-filter-user").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageBillingRecord.Create().
		SetUsageLogID(otherUsageLog.ID).
		SetBillingAccountID(otherAccount.ID).
		SetProjectID(project.ID).
		SetUserID(other.ID).
		SetModelID("gpt-admin-filter-other").
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("price-other").
		SetChargeAmountMicros(2_000_000).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusFailed).
		SetIdempotencyKey("usage:admin-filter-other").
		Save(ctx)
	require.NoError(t, err)

	ownerCtx := contexts.WithUser(ctx, owner)
	first := 10
	ledgerTxs, err := queryResolver.AdminLedgerTransactions(ownerCtx, &AdminLedgerTransactionsFilter{UserID: &user.ID, Direction: ptr(ledgertransaction.DirectionCredit)}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerTxs.TotalCount)
	require.Equal(t, userLedger.ID, ledgerTxs.Edges[0].Node.ID)

	usageRecords, err := queryResolver.AdminUsageBillingRecords(ownerCtx, &AdminUsageBillingRecordsFilter{UserID: &user.ID, ProjectID: &project.ID, ModelID: ptr("admin-filter"), Status: ptr(usagebillingrecord.StatusCharged)}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, usageRecords.TotalCount)
	require.Equal(t, user.ID, usageRecords.Edges[0].Node.UserID)

	paymentOrders, err := queryResolver.AdminPaymentOrders(ownerCtx, &AdminPaymentOrdersFilter{UserID: &user.ID, ProviderType: ptr(paymentorder.ProviderTypeEpay), Status: ptr(paymentorder.StatusPaid), ExternalTradeNo: ptr("admin_filter_user")}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, paymentOrders.TotalCount)
	require.Equal(t, userOrder.ID, paymentOrders.Edges[0].Node.ID)

	paymentEvents, err := queryResolver.AdminPaymentEvents(ownerCtx, &AdminPaymentEventsFilter{PaymentOrderID: &userOrder.ID, ProviderType: ptr(paymentevent.ProviderTypeEpay), Status: ptr(paymentevent.StatusProcessed), EventKey: ptr("filter_user")}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, paymentEvents.TotalCount)
	require.Equal(t, "evt_admin_filter_user", paymentEvents.Edges[0].Node.EventKey)
}

func TestBillingResolversOwnerCanQueryAdminBillingReportAndExportCSV(t *testing.T) {
	_, queryResolver, ctx, client, owner, project := setupBillingResolversTest(t, "billing_resolver_admin_report")
	user := createBillingResolverUser(t, ctx, client, false)
	account, err := queryResolver.billingAccountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(user.ID))
	require.NoError(t, err)
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	from := now.Add(-time.Hour)
	to := now.Add(time.Hour)

	_, err = client.LedgerTransaction.Create().
		SetCreatedAt(now).
		SetBillingAccountID(account.ID).
		SetDirection(ledgertransaction.DirectionCredit).
		SetAmountMicros(15_000_000).
		SetCurrency("CNY").
		SetType(ledgertransaction.TypePaymentRecharge).
		SetStatus(ledgertransaction.StatusPosted).
		SetIdempotencyKey("resolver-report-ledger").
		Save(ctx)
	require.NoError(t, err)

	usageLog := createBillingResolverUsageLog(t, ctx, client, project.ID, "gpt-report")
	_, err = client.UsageBillingRecord.Create().
		SetCreatedAt(now).
		SetUsageLogID(usageLog.ID).
		SetBillingAccountID(account.ID).
		SetProjectID(project.ID).
		SetUserID(user.ID).
		SetModelID("gpt-report").
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("resolver-report-price").
		SetChargeAmountMicros(4_000_000).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusCharged).
		SetIdempotencyKey("resolver-report-usage").
		Save(ctx)
	require.NoError(t, err)
	aggregateSvc := biz.NewUsageAggregateService(biz.UsageAggregateServiceParams{Ent: client})
	_, err = aggregateSvc.Rebuild(ctx, biz.UsageAggregateRebuildInput{})
	require.NoError(t, err)

	_, err = client.PaymentOrder.Create().
		SetCreatedAt(now).
		SetOrderNo("resolver-report-order").
		SetProjectID(project.ID).
		SetBillingAccountID(account.ID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(15_000_000).
		SetCurrency("CNY").
		SetStatus(paymentorder.StatusPaid).
		Save(ctx)
	require.NoError(t, err)

	ownerCtx := contexts.WithUser(ctx, owner)
	limit := 5
	report, err := queryResolver.AdminBillingReport(ownerCtx, &AdminBillingReportFilter{From: &from, To: &to, Currency: ptr("CNY"), Limit: &limit})
	require.NoError(t, err)
	require.Equal(t, int64(15_000_000), report.Summary.RechargeAmountMicros)
	require.Len(t, report.TopModels, 1)
	require.Equal(t, "gpt-report", report.TopModels[0].ModelID)

	csvPayload, err := queryResolver.ExportAdminBillingCSV(ownerCtx, ExportAdminBillingCSVInput{
		Dataset:  string(biz.BillingCSVExportDatasetUsageBillingRecords),
		From:     &from,
		To:       &to,
		Currency: ptr("CNY"),
		Limit:    &limit,
	})
	require.NoError(t, err)
	require.Equal(t, "text/csv", csvPayload.ContentType)
	require.Contains(t, csvPayload.FileName, "usage-billing-records")
	require.Contains(t, csvPayload.Content, "gpt-report")
}

func TestBillingResolversOwnerCanUpdateUserBillingAccount(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_resolver_owner_update_user_account")
	user := createBillingResolverUser(t, ctx, client, false)
	ownerCtx := contexts.WithUser(ctx, owner)

	account, err := mutationResolver.UpdateUserBillingAccount(ownerCtx, biz.UpdateUserBillingAccountInput{
		UserID:      user.ID,
		Status:      ptr(billingaccount.StatusFrozen),
		CreditLimit: ptr(decimal.RequireFromString("30")),
	})
	require.NoError(t, err)
	require.Equal(t, billingaccount.OwnerTypeUser, account.OwnerType)
	require.Equal(t, user.ID, account.OwnerID)
	require.Equal(t, billingaccount.StatusFrozen, account.Status)
	require.Equal(t, int64(30_000_000), account.CreditLimitMicros)

	loaded, err := queryResolver.UserBillingAccount(ownerCtx, objects.GUID{Type: ent.TypeUser, ID: user.ID})
	require.NoError(t, err)
	require.Equal(t, account.ID, loaded.ID)
	require.Equal(t, billingaccount.StatusFrozen, loaded.Status)
	require.Equal(t, int64(30_000_000), loaded.CreditLimitMicros)
}

func TestBillingResolversRejectsUserBillingAdminOperationsForNonOwner(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, _, _ := setupBillingResolversTest(t, "billing_resolver_admin_reject_non_owner")
	user := createBillingResolverUser(t, ctx, client, false)
	normalUser := createBillingResolverUser(t, ctx, client, false)
	userCtx := contexts.WithUser(ctx, normalUser)

	_, err := queryResolver.UserBillingAccount(userCtx, objects.GUID{Type: ent.TypeUser, ID: user.ID})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.AdjustUserBalance(userCtx, biz.AdjustUserBalanceInput{
		UserID:    user.ID,
		Direction: ledgertransaction.DirectionCredit,
		Amount:    decimal.NewFromInt(1),
	})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.UpdateUserBillingAccount(userCtx, biz.UpdateUserBillingAccountInput{
		UserID: user.ID,
		Status: ptr(billingaccount.StatusFrozen),
	})
	require.True(t, errors.Is(err, ErrNotOwner))

	first := 10
	_, err = queryResolver.AdminLedgerTransactions(userCtx, nil, nil, &first, nil, nil, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminUsageBillingRecords(userCtx, nil, nil, &first, nil, nil, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminPaymentOrders(userCtx, nil, nil, &first, nil, nil, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminPaymentEvents(userCtx, nil, nil, &first, nil, nil, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminBillingReport(userCtx, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.ExportAdminBillingCSV(userCtx, ExportAdminBillingCSVInput{Dataset: string(biz.BillingCSVExportDatasetLedgerTransactions)})
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminCommercialSetting(userCtx)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = queryResolver.AdminBillingAuditLogs(userCtx, nil, nil, &first, nil, nil, nil)
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = mutationResolver.SaveCommercialSetting(userCtx, biz.SaveCommercialSettingInput{Mode: commercialsetting.ModeEnforce})
	require.True(t, errors.Is(err, ErrNotOwner))
	_, err = mutationResolver.RunCommercialMaintenance(userCtx, RunCommercialMaintenanceInput{Reason: "non-owner maintenance"})
	require.True(t, errors.Is(err, ErrNotOwner))
}

func TestBillingResolversOwnerCanManageCommercialOperations(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_resolver_commercial_ops")
	ownerCtx := contexts.WithUser(ctx, owner)

	setting, err := queryResolver.AdminCommercialSetting(ownerCtx)
	require.NoError(t, err)
	require.Equal(t, commercialsetting.ModeEnforce, setting.Mode)

	updated, err := mutationResolver.SaveCommercialSetting(ownerCtx, biz.SaveCommercialSettingInput{
		Mode:                             commercialsetting.ModeWarn,
		RequireAdminActionReason:         true,
		PaymentProviderSecretsEncrypted:  true,
		WorkersEnabled:                   false,
		OrderExpiryWorkerEnabled:         true,
		HoldExpiryWorkerEnabled:          true,
		SubscriptionExpiryWorkerEnabled:  true,
		SubscriptionResetWorkerEnabled:   true,
		AffiliateRebateThawWorkerEnabled: true,
		FailedBillingRetryWorkerEnabled:  true,
		WorkerBatchSize:                  50,
		Currency:                         "CNY",
		Reason:                           "stage 10 production setting test",
	})
	require.NoError(t, err)
	require.Equal(t, setting.ID, updated.ID)
	require.Equal(t, commercialsetting.ModeWarn, updated.Mode)
	require.False(t, updated.WorkersEnabled)

	_, err = mutationResolver.RunCommercialMaintenance(ownerCtx, RunCommercialMaintenanceInput{})
	require.ErrorContains(t, err, "reason is required")
	result, err := mutationResolver.RunCommercialMaintenance(ownerCtx, RunCommercialMaintenanceInput{
		Reason: "stage 10 manual maintenance test",
	})
	require.NoError(t, err)
	require.Zero(t, result.OrderExpiryProcessed)
	require.Zero(t, result.HoldExpiryProcessed)

	first := 10
	action := "commercial_setting.save"
	logs, err := queryResolver.AdminBillingAuditLogs(ownerCtx, &AdminBillingAuditLogsFilter{Action: &action}, nil, &first, nil, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 1, logs.TotalCount)
	require.Equal(t, billingauditlog.ActorTypeAdmin, logs.Edges[0].Node.ActorType)
	require.Equal(t, owner.ID, *logs.Edges[0].Node.ActorUserID)
	require.Equal(t, "stage 10 production setting test", logs.Edges[0].Node.Reason)

	auditCount, err := client.BillingAuditLog.Query().
		Where(billingauditlog.ActionEQ("commercial_maintenance.run")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, auditCount)
}

func TestBillingResolversOwnerCanManageBillingPriceRules(t *testing.T) {
	mutationResolver, _, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_resolver_price_rules")
	ownerCtx := contexts.WithUser(ctx, owner)
	price := billingResolverModelPrice("1")
	enabled := true
	priority := 50
	referenceID := "global-gpt-test-v1"

	rule, err := mutationResolver.SaveBillingPriceRule(ownerCtx, SaveBillingPriceRuleForm{
		ScopeType:    billingpricerule.ScopeTypeGlobal,
		ScopeID:      0,
		ModelPattern: "gpt-test",
		Price:        &price,
		Priority:     &priority,
		Enabled:      &enabled,
		ReferenceID:  &referenceID,
	})
	require.NoError(t, err)
	require.Equal(t, billingpricerule.ScopeTypeGlobal, rule.ScopeType)
	require.Equal(t, "gpt-test", rule.ModelPattern)
	require.Equal(t, "global-gpt-test-v1", rule.ReferenceID)

	stored, err := client.BillingPriceRule.Get(ctx, rule.ID)
	require.NoError(t, err)
	require.Equal(t, 50, stored.Priority)

	updatedPrice := billingResolverModelPrice("2")
	updatedReferenceID := "global-gpt-test-v2"
	updated, err := mutationResolver.SaveBillingPriceRule(ownerCtx, SaveBillingPriceRuleForm{
		ID:           &objects.GUID{Type: ent.TypeBillingPriceRule, ID: rule.ID},
		ScopeType:    billingpricerule.ScopeTypeGlobal,
		ScopeID:      0,
		ModelPattern: "gpt-test",
		Price:        &updatedPrice,
		Priority:     &priority,
		Enabled:      &enabled,
		ReferenceID:  &updatedReferenceID,
	})
	require.NoError(t, err)
	require.Equal(t, rule.ID, updated.ID)
	require.Equal(t, "global-gpt-test-v2", updated.ReferenceID)

	deleted, err := mutationResolver.DeleteBillingPriceRule(ownerCtx, objects.GUID{Type: ent.TypeBillingPriceRule, ID: rule.ID})
	require.NoError(t, err)
	require.True(t, deleted)

	_, err = client.BillingPriceRule.Get(ctx, rule.ID)
	require.True(t, ent.IsNotFound(err))
}

func TestBillingResolversRejectsPriceRuleManagementForNonOwner(t *testing.T) {
	mutationResolver, _, ctx, client, _, _ := setupBillingResolversTest(t, "billing_resolver_price_rules_non_owner")
	normalUser := createBillingResolverUser(t, ctx, client, false)
	userCtx := contexts.WithUser(ctx, normalUser)
	price := billingResolverModelPrice("1")

	_, err := mutationResolver.SaveBillingPriceRule(userCtx, SaveBillingPriceRuleForm{
		ScopeType:    billingpricerule.ScopeTypeGlobal,
		ScopeID:      0,
		ModelPattern: "gpt-test",
		Price:        &price,
	})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.DeleteBillingPriceRule(userCtx, objects.GUID{Type: ent.TypeBillingPriceRule, ID: 1})
	require.True(t, errors.Is(err, ErrNotOwner))
}

func TestBillingResolversUserCanPurchaseAndReadOwnSubscriptionPlan(t *testing.T) {
	mutationResolver, _, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_resolver_user_purchase_subscription")
	defer client.Close()

	ownerCtx := contexts.WithUser(ctx, owner)
	plan, err := mutationResolver.SaveSubscriptionPlan(ownerCtx, biz.SaveSubscriptionPlanInput{
		Name:           "Self-service plan",
		Period:         "month",
		PeriodDays:     30,
		Price:          decimal.Zero,
		Currency:       "CNY",
		IncludedAmount: decimal.RequireFromString("20"),
		Status:         "enabled",
	})
	require.NoError(t, err)

	normalUser := createBillingResolverUser(t, ctx, client, false)
	userCtx := contexts.WithUser(context.Background(), normalUser)

	subscription, err := mutationResolver.PurchaseSubscriptionPlan(userCtx, biz.PurchaseSubscriptionPlanInput{PlanID: plan.ID})
	require.NoError(t, err)
	require.Equal(t, normalUser.ID, subscription.UserID)

	reloaded, err := client.UserSubscription.Get(userCtx, subscription.ID)
	require.NoError(t, err)
	require.Equal(t, subscription.ID, reloaded.ID)

	readablePlan, err := reloaded.QueryPlan().Only(userCtx)
	require.NoError(t, err)
	require.Equal(t, plan.ID, readablePlan.ID)
}

func TestBillingGraphQLUserCanPurchaseSubscriptionPlanSelection(t *testing.T) {
	mutationResolver, _, ctx, client, owner, _ := setupBillingResolversTest(t, "billing_graphql_user_purchase_subscription")
	defer client.Close()

	ownerCtx := contexts.WithUser(ctx, owner)
	plan, err := mutationResolver.SaveSubscriptionPlan(ownerCtx, biz.SaveSubscriptionPlanInput{
		Name:           "GraphQL self-service plan",
		Period:         "month",
		PeriodDays:     30,
		Price:          decimal.Zero,
		Currency:       "CNY",
		IncludedAmount: decimal.RequireFromString("20"),
		Status:         "enabled",
	})
	require.NoError(t, err)

	normalUser := createBillingResolverUser(t, ctx, client, false)
	gqlSrv := handler.NewDefaultServer(NewExecutableSchema(Config{Resolvers: mutationResolver.Resolver}))
	gqlSrv.Use(entgql.Transactioner{TxOpener: client})
	graphQLHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gqlSrv.ServeHTTP(w, r.WithContext(contexts.WithUser(r.Context(), normalUser)))
	})

	body, err := json.Marshal(map[string]any{
		"query": `
			mutation PurchaseSubscriptionPlan($input: PurchaseSubscriptionPlanInput!) {
				purchaseSubscriptionPlan(input: $input) {
					id
					status
					startsAt
					expiresAt
					includedAmountMicros
					usedAmountMicros
					currency
					plan {
						id
						name
						period
						priceMicros
						currency
					}
				}
			}`,
		"variables": map[string]any{
			"input": map[string]any{
				"planId": fmt.Sprintf("gid://axonhub/%s/%d", ent.TypeSubscriptionPlan, plan.ID),
			},
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/admin/graphql", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	graphQLHandler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var payload struct {
		Data struct {
			PurchaseSubscriptionPlan struct {
				Status string `json:"status"`
				Plan   struct {
					Name string `json:"name"`
				} `json:"plan"`
			} `json:"purchaseSubscriptionPlan"`
		} `json:"data"`
		Errors []map[string]any `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	require.Empty(t, payload.Errors, rec.Body.String())
	require.Equal(t, "active", payload.Data.PurchaseSubscriptionPlan.Status)
	require.Equal(t, plan.Name, payload.Data.PurchaseSubscriptionPlan.Plan.Name)
}

func setupBillingResolversTest(t *testing.T, name string) (*mutationResolver, *queryResolver, context.Context, *ent.Client, *ent.User, *ent.Project) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())

	owner := createBillingResolverUser(t, ctx, client, true)
	project, err := client.Project.Create().
		SetName(name).
		SetStatus(entproject.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	billingAccountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	billingAuditSvc := biz.NewBillingAuditService(biz.BillingAuditServiceParams{Ent: client})
	ledgerSvc := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	pricingSvc := biz.NewPricingService(biz.PricingServiceParams{Ent: client})
	paymentSvc := biz.NewPaymentService(biz.PaymentServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
		ProviderRegistry:      biz.NewPaymentProviderRegistry(),
	})
	commercialOperationsSvc := biz.NewCommercialOperationsService(biz.CommercialOperationsServiceParams{
		Ent:                 client,
		PaymentService:      paymentSvc,
		BillingAuditService: billingAuditSvc,
	})
	subscriptionSvc := biz.NewSubscriptionService(biz.SubscriptionServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
	})

	resolver := &Resolver{
		client:                      client,
		billingAccountService:       billingAccountSvc,
		billingAuditService:         billingAuditSvc,
		commercialOperationsService: commercialOperationsSvc,
		paymentService:              paymentSvc,
		pricingService:              pricingSvc,
		subscriptionService:         subscriptionSvc,
	}

	return &mutationResolver{resolver}, &queryResolver{resolver}, ctx, client, owner, project
}

func ptr[T any](value T) *T {
	return &value
}

func createBillingResolverUser(t *testing.T, ctx context.Context, client *ent.Client, isOwner bool) *ent.User {
	t.Helper()

	password, err := biz.HashPassword("test-password")
	require.NoError(t, err)

	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("billing-%t-%d@example.com", isOwner, time.Now().UnixNano())).
		SetPassword(password).
		SetFirstName("Billing").
		SetLastName("Tester").
		SetStatus(entuser.StatusActivated).
		SetIsOwner(isOwner).
		Save(ctx)
	require.NoError(t, err)

	return user
}

func createBillingResolverUsageLog(t *testing.T, ctx context.Context, client *ent.Client, projectID int, modelID string) *ent.UsageLog {
	t.Helper()

	req, err := client.Request.Create().
		SetProjectID(projectID).
		SetModelID(modelID).
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetProjectID(projectID).
		SetModelID(modelID).
		SetPromptTokens(1).
		SetCompletionTokens(1).
		SetTotalTokens(2).
		Save(ctx)
	require.NoError(t, err)

	return usageLog
}

func billingResolverModelPrice(unitPrice string) objects.ModelPrice {
	price := decimal.RequireFromString(unitPrice)
	return objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: &price,
				},
			},
		},
	}
}
