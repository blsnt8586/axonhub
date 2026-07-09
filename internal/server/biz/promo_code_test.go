package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/promocode"
	"github.com/looplj/axonhub/internal/ent/promousage"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
)

func TestPromoCodeRechargeDiscountUsesPayableForPaymentAndOriginalForWalletCredit(t *testing.T) {
	t.Parallel()

	client, ctx, paymentSvc := newPaymentTestService(t, "promo_recharge_discount")
	promoSvc := NewPromoCodeService(PromoCodeServiceParams{Ent: client})
	paymentSvc.promoCodeService = promoSvc
	user := createPromoTestUser(t, client, ctx, "promo-recharge@example.com")
	provider, err := paymentSvc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)
	_, err = promoSvc.Save(ctx, SavePromoCodeInput{
		Code:           "SAVE20",
		DiscountType:   promocode.DiscountTypeAmount,
		DiscountAmount: decimal.RequireFromString("20"),
		Scope:          promocode.ScopeRecharge,
		Currency:       "CNY",
	})
	require.NoError(t, err)

	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		BillingSubject:     UserBillingSubject(user.ID),
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("100"),
		Currency:           "CNY",
		PromoCode:          "save-20",
	})
	require.NoError(t, err)
	require.Equal(t, "80.00", checkout.Params["money"])

	order, err := client.PaymentOrder.Query().Where(paymentorder.OrderNoEQ(checkout.OrderNo)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), order.AmountMicros)
	require.Equal(t, int64(20_000_000), order.DiscountAmountMicros)
	require.Equal(t, int64(80_000_000), order.PayableAmountMicros)

	badNotify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	badNotify["money"] = "100.00"
	badNotify["sign"] = SignEPayParams(badNotify, "axonhub-simulated-epay-secret")
	_, err = paymentSvc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: badNotify})
	require.ErrorContains(t, err, "epay money mismatch")

	notify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	paid, err := paymentSvc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)
	require.Equal(t, paymentorder.StatusPaid, paid.Status)

	account, err := client.BillingAccount.Get(ctx, paid.BillingAccountID)
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), account.BalanceMicros)
	usage, err := client.PromoUsage.Query().Where(promousage.PaymentOrderIDEQ(paid.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, promousage.StatusApplied, usage.Status)
	require.Equal(t, int64(80_000_000), usage.PayableAmountMicros)
}

func TestPromoCodeSubscriptionDiscountDebitsPayableAmount(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "promo_subscription_discount")
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err := ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("100"), ledgertransaction.TypePaymentRecharge, "promo-sub-credit")
	require.NoError(t, err)
	promoSvc := NewPromoCodeService(PromoCodeServiceParams{Ent: client})
	subscriptionSvc := newSubscriptionTestService(client)
	subscriptionSvc.promoCodeService = promoSvc
	processor.subscriptionService = subscriptionSvc
	_, err = promoSvc.Save(ctx, SavePromoCodeInput{
		Code:               "PLAN10",
		DiscountType:       promocode.DiscountTypePercent,
		DiscountPercentBps: 1000,
		Scope:              promocode.ScopeSubscription,
		Currency:           "CNY",
	})
	require.NoError(t, err)
	plan, err := subscriptionSvc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:           "Discountable",
		PeriodDays:     30,
		Price:          decimal.RequireFromString("50"),
		IncludedAmount: decimal.RequireFromString("100"),
		Currency:       "CNY",
	})
	require.NoError(t, err)

	sub, err := subscriptionSvc.PurchasePlan(ctx, PurchaseSubscriptionPlanInput{
		UserID:    account.OwnerID,
		PlanID:    plan.ID,
		Now:       time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
		PromoCode: "PLAN10",
	})
	require.NoError(t, err)
	require.Equal(t, usersubscription.StatusActive, sub.Status)
	require.Equal(t, int64(50_000_000), sub.OriginalPriceMicros)
	require.Equal(t, int64(5_000_000), sub.DiscountAmountMicros)
	require.Equal(t, int64(45_000_000), sub.PayableAmountMicros)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(55_000_000), reloaded.BalanceMicros)
	usage, err := client.PromoUsage.Query().Where(promousage.UserSubscriptionIDEQ(sub.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, promousage.StatusApplied, usage.Status)
}

func TestPromoCodeValidationRejectsInvalidLifecycleAndLimits(t *testing.T) {
	t.Parallel()

	client, ctx, promoSvc, user := newPromoCodeTestService(t, "promo_validation")
	user2 := createPromoTestUser(t, client, ctx, "promo-validation-2@example.com")

	_, err := promoSvc.Save(ctx, SavePromoCodeInput{Code: "OFF", DiscountType: promocode.DiscountTypeAmount, DiscountAmount: decimal.RequireFromString("1"), Status: promocode.StatusDisabled})
	require.NoError(t, err)
	_, err = promoSvc.Quote(ctx, promoQuoteForUser("OFF", user.ID))
	require.ErrorIs(t, err, ErrPromoCodeDisabled)

	expired, err := promoSvc.Save(ctx, SavePromoCodeInput{Code: "OLD", DiscountType: promocode.DiscountTypeAmount, DiscountAmount: decimal.RequireFromString("1")})
	require.NoError(t, err)
	_, err = promoSvc.UpdateStatus(ctx, UpdatePromoCodeStatusInput{CodeID: expired.ID, Status: promocode.StatusExpired})
	require.NoError(t, err)
	_, err = promoSvc.Quote(ctx, promoQuoteForUser("OLD", user.ID))
	require.ErrorIs(t, err, ErrPromoCodeExpired)

	_, err = promoSvc.Save(ctx, SavePromoCodeInput{Code: "ONCE", DiscountType: promocode.DiscountTypeAmount, DiscountAmount: decimal.RequireFromString("1"), MaxUses: 1, PerUserLimit: 0})
	require.NoError(t, err)
	_, err = promoSvc.Apply(ctx, promoApplyForUser("ONCE", user.ID, "promo-once-1"))
	require.NoError(t, err)
	_, err = promoSvc.Apply(ctx, promoApplyForUser("ONCE", user2.ID, "promo-once-2"))
	require.ErrorIs(t, err, ErrPromoCodeExhausted)

	_, err = promoSvc.Save(ctx, SavePromoCodeInput{Code: "REUSE", DiscountType: promocode.DiscountTypeAmount, DiscountAmount: decimal.RequireFromString("1"), PerUserLimit: 1})
	require.NoError(t, err)
	_, err = promoSvc.Apply(ctx, promoApplyForUser("REUSE", user.ID, "promo-reuse-1"))
	require.NoError(t, err)
	_, err = promoSvc.Apply(ctx, promoApplyForUser("REUSE", user.ID, "promo-reuse-2"))
	require.ErrorIs(t, err, ErrPromoCodeReused)

	_, err = promoSvc.Save(ctx, SavePromoCodeInput{Code: "CAP", DiscountType: promocode.DiscountTypeAmount, DiscountAmount: decimal.RequireFromString("500"), PerUserLimit: 0})
	require.NoError(t, err)
	quote, err := promoSvc.Quote(ctx, promoQuoteForUser("CAP", user2.ID))
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), quote.DiscountAmountMicros)
	require.Zero(t, quote.PayableAmountMicros)
}

func newPromoCodeTestService(t *testing.T, name string) (*ent.Client, context.Context, *PromoCodeService, *ent.User) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	user := createPromoTestUser(t, client, ctx, name+"@example.com")
	return client, ctx, NewPromoCodeService(PromoCodeServiceParams{Ent: client}), user
}

func createPromoTestUser(t *testing.T, client *ent.Client, ctx context.Context, email string) *ent.User {
	t.Helper()
	user, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return user
}

func promoQuoteForUser(code string, userID int) PromoQuoteInput {
	return PromoQuoteInput{Code: code, Scope: promousage.ScopeRecharge, UserID: userID, OriginalAmountMicros: 100_000_000, Currency: "CNY"}
}

func promoApplyForUser(code string, userID int, key string) PromoApplyInput {
	return PromoApplyInput{PromoQuoteInput: promoQuoteForUser(code, userID), IdempotencyKey: fmt.Sprintf("%s:%s", key, code)}
}
