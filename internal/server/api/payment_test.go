package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
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

	req := httptest.NewRequest(http.MethodGet, checkout.URL, nil).WithContext(ctx)
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
