package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	entproject "github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPaymentServiceManualRechargeCreditsLedgerOnce(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newPaymentTestService(t, "payment_manual_recharge")
	order, err := svc.CreateManualRechargeOrder(ctx, CreateManualRechargeOrderInput{
		ProjectID: 1,
		Amount:    decimal.RequireFromString("10"),
		Currency:  "CNY",
		Metadata:  objects.JSONRawMessage([]byte(`{"source":"test"}`)),
	})
	require.NoError(t, err)
	require.Equal(t, paymentorder.StatusPending, order.Status)
	require.Equal(t, int64(10_000_000), order.AmountMicros)

	paid, err := svc.ConfirmManualPayment(ctx, ConfirmManualPaymentInput{
		OrderNo:         order.OrderNo,
		EventKey:        "manual-paid-once",
		ExternalTradeNo: "offline-001",
		ActorID:         "admin-1",
		Payload:         objects.JSONRawMessage([]byte(`{"paid":true}`)),
	})
	require.NoError(t, err)
	require.Equal(t, paymentorder.StatusPaid, paid.Status)
	require.NotNil(t, paid.PaidAt)
	require.NotNil(t, paid.LedgerTransactionID)
	require.NotNil(t, paid.ExternalTradeNo)
	require.Equal(t, "offline-001", *paid.ExternalTradeNo)

	account, err := client.BillingAccount.Get(ctx, paid.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(10_000_000), account.BalanceMicros)

	again, err := svc.ConfirmManualPayment(ctx, ConfirmManualPaymentInput{
		OrderNo:  order.OrderNo,
		EventKey: "manual-paid-once",
		ActorID:  "admin-1",
	})
	require.NoError(t, err)
	require.Equal(t, paid.ID, again.ID)

	account, err = client.BillingAccount.Get(ctx, paid.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(10_000_000), account.BalanceMicros)

	ledgerCount, err := client.LedgerTransaction.Query().
		Where(ledgertransaction.IdempotencyKeyEQ(paymentLedgerIdempotencyKey(order.ID))).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerCount)

	eventCount, err := client.PaymentEvent.Query().
		Where(paymentevent.EventKeyEQ("manual-paid-once")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, eventCount)
}

func TestPaymentServiceRejectsEventKeyReusedByAnotherOrder(t *testing.T) {
	t.Parallel()

	_, ctx, svc := newPaymentTestService(t, "payment_event_key_reuse")
	first, err := svc.CreateManualRechargeOrder(ctx, CreateManualRechargeOrderInput{
		ProjectID: 1,
		Amount:    decimal.RequireFromString("1"),
	})
	require.NoError(t, err)
	second, err := svc.CreateManualRechargeOrder(ctx, CreateManualRechargeOrderInput{
		ProjectID: 1,
		Amount:    decimal.RequireFromString("2"),
	})
	require.NoError(t, err)

	_, err = svc.ConfirmManualPayment(ctx, ConfirmManualPaymentInput{
		OrderNo:  first.OrderNo,
		EventKey: "same-provider-event",
	})
	require.NoError(t, err)

	_, err = svc.ConfirmManualPayment(ctx, ConfirmManualPaymentInput{
		OrderNo:  second.OrderNo,
		EventKey: "same-provider-event",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not belong to order")
}

func newPaymentTestService(t *testing.T, name string) (*ent.Client, context.Context, *PaymentService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	_, err := client.Project.Create().
		SetName(name).
		SetStatus(entproject.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	svc := NewPaymentService(PaymentServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
	})

	return client, ctx, svc
}
