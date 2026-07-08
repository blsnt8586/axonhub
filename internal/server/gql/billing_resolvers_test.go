package gql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	entproject "github.com/looplj/axonhub/internal/ent/project"
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
	require.Equal(t, "secret-key", cfg["key"])
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
	ledgerSvc := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	paymentSvc := biz.NewPaymentService(biz.PaymentServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
		ProviderRegistry:      biz.NewPaymentProviderRegistry(),
	})

	resolver := &Resolver{
		client:                client,
		billingAccountService: billingAccountSvc,
		paymentService:        paymentSvc,
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
