package biz

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
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

func TestPaymentServiceSimulatedEPayCheckoutAndNotifyCreditsLedgerOnce(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newPaymentTestService(t, "payment_epay_notify")
	provider, err := svc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)

	checkout, err := svc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("12.34"),
		Currency:           "CNY",
		Subject:            "Test recharge",
	})
	require.NoError(t, err)
	require.Equal(t, "redirect", checkout.Method)
	require.Contains(t, checkout.URL, "/payment/simulate/epay/submit?")
	require.Equal(t, "12.34", checkout.Params["money"])

	notify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	paid, err := svc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)
	require.Equal(t, paymentorder.StatusPaid, paid.Status)
	require.NotNil(t, paid.LedgerTransactionID)
	require.NotNil(t, paid.ExternalTradeNo)
	require.True(t, strings.HasPrefix(*paid.ExternalTradeNo, "sim_"))

	account, err := client.BillingAccount.Get(ctx, paid.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(12_340_000), account.BalanceMicros)

	again, err := svc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)
	require.Equal(t, paid.ID, again.ID)

	account, err = client.BillingAccount.Get(ctx, paid.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(12_340_000), account.BalanceMicros)

	ledger, err := client.LedgerTransaction.Get(ctx, *paid.LedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.CreatedByTypeProvider, ledger.CreatedByType)
	require.Equal(t, fmt.Sprint(provider.ID), ledger.CreatedByID)

	eventCount, err := client.PaymentEvent.Query().
		Where(paymentevent.EventKeyEQ("epay_notify:" + notify["trade_no"])).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, eventCount)
}

func TestPaymentServiceRejectsEPayNotifyWithInvalidSignature(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newPaymentTestService(t, "payment_epay_bad_sign")
	provider, err := svc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)

	checkout, err := svc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("1"),
	})
	require.NoError(t, err)

	notify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	notify["money"] = "99.00"
	_, err = svc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid epay signature")

	failedKey := fmt.Sprintf("epay_notify_failed:%s:invalid_signature", notify["trade_no"])
	event, err := client.PaymentEvent.Query().
		Where(paymentevent.EventKeyEQ(failedKey)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, paymentevent.StatusFailed, event.Status)
	require.Equal(t, "epay_notify_failed", event.EventType)
	require.Contains(t, event.Error, "invalid epay signature")
	require.NotNil(t, event.PaymentOrderID)
	require.NotNil(t, event.ProviderInstanceID)

	_, err = svc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.Error(t, err)

	eventCount, err := client.PaymentEvent.Query().
		Where(paymentevent.EventKeyEQ(failedKey)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, eventCount)

	ledgerCount, err := client.LedgerTransaction.Query().
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, ledgerCount)
}

func TestPaymentServiceRecordsEPayMoneyMismatchWithValidSignature(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newPaymentTestService(t, "payment_epay_money_mismatch")
	provider, err := svc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)

	checkout, err := svc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("1"),
	})
	require.NoError(t, err)

	notify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	notify["money"] = "99.00"
	notify["sign"] = SignEPayParams(notify, "axonhub-simulated-epay-secret")

	_, err = svc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.Error(t, err)
	require.Contains(t, err.Error(), "epay money mismatch")

	failedKey := fmt.Sprintf("epay_notify_failed:%s:money_mismatch", notify["trade_no"])
	event, err := client.PaymentEvent.Query().
		Where(paymentevent.EventKeyEQ(failedKey)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, paymentevent.StatusFailed, event.Status)
	require.Contains(t, event.Error, "epay money mismatch")

	ledgerCount, err := client.LedgerTransaction.Query().
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, ledgerCount)
}

func TestPaymentServiceUpsertEPayProviderCreatesAndPreservesKeyOnUpdate(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newPaymentTestService(t, "payment_epay_provider_upsert")
	provider, err := svc.UpsertEPayProvider(ctx, UpsertEPayProviderInput{
		Name:       "Prod ePay",
		GatewayURL: "https://pay.example.com/submit.php",
		PID:        "1002",
		Key:        "secret-key-v1",
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/admin/billing",
		Type:       "wxpay",
		SiteName:   "AxonHub Prod",
	})
	require.NoError(t, err)
	require.Equal(t, "Prod ePay", provider.Name)
	require.Equal(t, paymentproviderinstance.ProviderTypeEpay, provider.ProviderType)
	require.Equal(t, paymentproviderinstance.StatusEnabled, provider.Status)
	require.Equal(t, "CNY", provider.Currency)

	cfg, err := parseEPayConfig(provider.Config)
	require.NoError(t, err)
	require.Equal(t, "secret-key-v1", cfg.Key)
	require.Equal(t, "wxpay", cfg.Type)

	updated, err := svc.UpsertEPayProvider(ctx, UpsertEPayProviderInput{
		Name:       "Prod ePay",
		Status:     paymentproviderinstance.StatusDisabled,
		Currency:   "CNY",
		GatewayURL: "https://pay2.example.com/submit.php",
		PID:        "1002",
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/admin/billing",
	})
	require.NoError(t, err)
	require.Equal(t, provider.ID, updated.ID)
	require.Equal(t, paymentproviderinstance.StatusDisabled, updated.Status)

	cfg, err = parseEPayConfig(updated.Config)
	require.NoError(t, err)
	require.Equal(t, "secret-key-v1", cfg.Key)
	require.Equal(t, "https://pay2.example.com/submit.php", cfg.GatewayURL)
	require.Equal(t, "alipay", cfg.Type)
	require.Equal(t, "AxonHub", cfg.SiteName)

	count, err := client.PaymentProviderInstance.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestPaymentServiceUpsertEPayProviderRejectsMissingInitialKey(t *testing.T) {
	t.Parallel()

	_, ctx, svc := newPaymentTestService(t, "payment_epay_provider_missing_key")
	_, err := svc.UpsertEPayProvider(ctx, UpsertEPayProviderInput{
		Name:       "Prod ePay",
		GatewayURL: "https://pay.example.com/submit.php",
		PID:        "1002",
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/admin/billing",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "epay key is required")
}

func TestPaymentServiceUpsertEPayProviderRejectsReservedSimulationName(t *testing.T) {
	t.Parallel()

	_, ctx, svc := newPaymentTestService(t, "payment_epay_provider_reserved_name")
	_, err := svc.UpsertEPayProvider(ctx, UpsertEPayProviderInput{Name: simulatedEPayProviderName})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
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
		ProviderRegistry:      NewPaymentProviderRegistry(),
	})

	return client, ctx, svc
}
