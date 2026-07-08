package gql

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
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
	})

	resolver := &Resolver{
		client:                client,
		billingAccountService: billingAccountSvc,
		paymentService:        paymentSvc,
	}

	return &mutationResolver{resolver}, &queryResolver{resolver}, ctx, client, owner, project
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
