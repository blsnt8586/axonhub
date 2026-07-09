package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	entproject "github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestPaymentHandlersSimulateEPaySubmitCreditsOrder(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, paymentSvc := newPaymentAPITestService(t, "api_payment_simulated_epay")
	provider, err := paymentSvc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)

	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, biz.CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("5.67"),
		Currency:           "CNY",
	})
	require.NoError(t, err)

	router := gin.New()
	router.GET("/payment/simulate/epay/submit", NewPaymentHandlers(PaymentHandlersParams{PaymentService: paymentSvc}).SimulateEPaySubmit)

	req := httptest.NewRequest(http.MethodGet, checkout.URL, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusFound, w.Code)

	order, err := client.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, order.LedgerTransactionID)

	account, err := client.BillingAccount.Get(ctx, order.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(5_670_000), account.BalanceMicros)
}

func TestPaymentHandlersSimulateEPaySubmitUsesOrderProviderKey(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, paymentSvc := newPaymentAPITestService(t, "api_payment_simulated_epay_custom_key")
	provider, err := paymentSvc.UpsertEPayProvider(ctx, biz.UpsertEPayProviderInput{
		Name:       "Custom ePay",
		GatewayURL: "http://axon.local/payment/simulate/epay/submit",
		PID:        "custom-pid",
		Key:        "custom-provider-key",
		NotifyURL:  "http://axon.local/payment/notify/epay",
		ReturnURL:  "http://axon.local/payment/return/epay",
		Currency:   "CNY",
	})
	require.NoError(t, err)

	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, biz.CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("8.90"),
		Currency:           "CNY",
	})
	require.NoError(t, err)

	router := gin.New()
	router.GET("/payment/simulate/epay/submit", NewPaymentHandlers(PaymentHandlersParams{PaymentService: paymentSvc}).SimulateEPaySubmit)

	req := httptest.NewRequest(http.MethodGet, checkout.URL, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusFound, w.Code)

	order, err := client.PaymentOrder.Query().Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, order.LedgerTransactionID)

	account, err := client.BillingAccount.Get(ctx, order.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(8_900_000), account.BalanceMicros)
}

func TestPaymentHandlersReturnEPayReportsOrderStatusWithoutCrediting(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, paymentSvc := newPaymentAPITestService(t, "api_payment_epay_return")
	provider, err := paymentSvc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)

	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, biz.CreateRechargeCheckoutInput{
		ProjectID:          1,
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("3.21"),
		Currency:           "CNY",
	})
	require.NoError(t, err)

	returnParams := biz.NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	returnURL, err := appendEPayReturnParams("/payment/return/epay", returnParams)
	require.NoError(t, err)

	router := gin.New()
	handlers := NewPaymentHandlers(PaymentHandlersParams{PaymentService: paymentSvc})
	router.GET("/payment/return/epay", handlers.ReturnEPay)

	req := httptest.NewRequest(http.MethodGet, returnURL, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, false, body["paid"])
	require.Equal(t, string(paymentorder.StatusPending), body["order_status"])

	ledgerCount, err := client.LedgerTransaction.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, ledgerCount)

	_, err = paymentSvc.HandleEPayNotify(ctx, biz.HandleEPayNotifyInput{Params: returnParams})
	require.NoError(t, err)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	body = map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, true, body["paid"])
	require.Equal(t, string(paymentorder.StatusPaid), body["order_status"])
}

func newPaymentAPITestService(t *testing.T, name string) (*ent.Client, context.Context, *biz.PaymentService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	_, err := client.Project.Create().
		SetName(name).
		SetStatus(entproject.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	accountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	ledgerSvc := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	return client, ctx, biz.NewPaymentService(biz.PaymentServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
		ProviderRegistry:      biz.NewPaymentProviderRegistry(),
	})
}
