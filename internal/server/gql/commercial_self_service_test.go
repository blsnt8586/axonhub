package gql

import (
	"net/url"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/affiliaterebate"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/promocode"
	"github.com/looplj/axonhub/internal/ent/promousage"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestCommercialSelfServiceRechargePromoAndAffiliateClosure(t *testing.T) {
	mutationResolver, queryResolver, setupCtx, client, owner, _ := setupBillingResolversTest(t, "commercial_self_service_recharge_affiliate")
	ownerCtx := contexts.WithUser(setupCtx, owner)

	_, err := mutationResolver.SavePromoCode(ownerCtx, biz.SavePromoCodeInput{
		Code:           "SAVE20",
		DiscountType:   promocode.DiscountTypeAmount,
		DiscountAmount: decimal.RequireFromString("20"),
		Scope:          promocode.ScopeRecharge,
		Currency:       "CNY",
	})
	require.NoError(t, err)
	_, err = mutationResolver.SaveAffiliateSetting(ownerCtx, biz.SaveAffiliateSettingInput{
		Enabled:              true,
		DefaultRebateRateBps: 1000,
		FreezeDays:           0,
		Currency:             "CNY",
	})
	require.NoError(t, err)

	inviter := createBillingResolverUser(t, setupCtx, client, false)
	invitee := createBillingResolverUser(t, setupCtx, client, false)
	inviterCtx := contexts.WithUser(setupCtx, inviter)
	inviteeCtx := contexts.WithUser(setupCtx, invitee)
	inviterSummary, err := queryResolver.MyAffiliateSummary(inviterCtx)
	require.NoError(t, err)
	invitation, err := mutationResolver.BindAffiliateInvite(inviteeCtx, biz.BindAffiliateInviteInput{
		InviteCode: inviterSummary.Profile.InviteCode,
	})
	require.NoError(t, err)
	require.Equal(t, inviter.ID, invitation.InviterUserID)
	require.Equal(t, invitee.ID, invitation.InviteeUserID)

	provider, err := mutationResolver.paymentService.GetOrCreateSimulatedEPayProvider(setupCtx, "https://axon.example.com")
	require.NoError(t, err)
	promoCode := "save20"
	checkout, err := mutationResolver.CreateMyEPayRechargeCheckout(inviteeCtx, CreateMyEPayRechargeCheckoutInput{
		Amount:             decimal.RequireFromString("100"),
		Currency:           ptr("CNY"),
		PromoCode:          &promoCode,
		ProviderInstanceID: &objects.GUID{Type: ent.TypePaymentProviderInstance, ID: provider.ID},
	})
	require.NoError(t, err)
	require.NotNil(t, checkout.URL)

	checkoutParams := checkoutParamsFromURL(t, *checkout.URL)
	require.Equal(t, "80.00", checkoutParams["money"])
	order, err := client.PaymentOrder.Query().Where(paymentorder.OrderNoEQ(checkout.OrderNo)).Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), order.AmountMicros)
	require.Equal(t, int64(20_000_000), order.DiscountAmountMicros)
	require.Equal(t, int64(80_000_000), order.PayableAmountMicros)

	notify := biz.NewSimulatedEPayNotifyFromCheckout(checkoutParams, "axonhub-simulated-epay-secret")
	paid, err := mutationResolver.paymentService.HandleEPayNotify(setupCtx, biz.HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)
	_, err = mutationResolver.paymentService.HandleEPayNotify(setupCtx, biz.HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)

	inviteeAccount, err := queryResolver.MyBillingAccount(inviteeCtx)
	require.NoError(t, err)
	require.Equal(t, int64(100_000_000), inviteeAccount.BalanceMicros)
	promoUsage, err := client.PromoUsage.Query().Where(promousage.PaymentOrderIDEQ(paid.ID)).Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, promousage.StatusApplied, promoUsage.Status)
	require.Equal(t, int64(80_000_000), promoUsage.PayableAmountMicros)

	rebates, err := client.AffiliateRebate.Query().Where(affiliaterebate.PaymentOrderIDEQ(paid.ID)).All(setupCtx)
	require.NoError(t, err)
	require.Len(t, rebates, 1)
	require.Equal(t, int64(8_000_000), rebates[0].AmountMicros)

	transfer, err := mutationResolver.TransferAffiliateRebates(inviterCtx)
	require.NoError(t, err)
	require.Equal(t, 1, transfer.TransferredCount)
	require.Equal(t, int64(8_000_000), transfer.TransferredMicros)
	transferAgain, err := mutationResolver.TransferAffiliateRebates(inviterCtx)
	require.NoError(t, err)
	require.Zero(t, transferAgain.TransferredCount)
	require.Zero(t, transferAgain.TransferredMicros)

	inviterAccount, err := queryResolver.MyBillingAccount(inviterCtx)
	require.NoError(t, err)
	require.Equal(t, int64(8_000_000), inviterAccount.BalanceMicros)
}

func TestCommercialSelfServiceSubscriptionPromoClosure(t *testing.T) {
	mutationResolver, queryResolver, setupCtx, client, owner, _ := setupBillingResolversTest(t, "commercial_self_service_subscription_promo")
	ownerCtx := contexts.WithUser(setupCtx, owner)
	user := createBillingResolverUser(t, setupCtx, client, false)
	userCtx := contexts.WithUser(setupCtx, user)

	_, err := mutationResolver.AdjustUserBalance(ownerCtx, biz.AdjustUserBalanceInput{
		UserID:         user.ID,
		Direction:      "credit",
		Amount:         decimal.RequireFromString("100"),
		Currency:       "CNY",
		IdempotencyKey: "subscription-promo-credit",
		Memo:           "subscription promo acceptance",
	})
	require.NoError(t, err)
	_, err = mutationResolver.SavePromoCode(ownerCtx, biz.SavePromoCodeInput{
		Code:               "PLAN10",
		DiscountType:       promocode.DiscountTypePercent,
		DiscountPercentBps: 1000,
		Scope:              promocode.ScopeSubscription,
		Currency:           "CNY",
	})
	require.NoError(t, err)
	plan, err := mutationResolver.SaveSubscriptionPlan(ownerCtx, biz.SaveSubscriptionPlanInput{
		Name:           "Discount Plan",
		PeriodDays:     30,
		Price:          decimal.RequireFromString("50"),
		IncludedAmount: decimal.RequireFromString("100"),
		Currency:       "CNY",
	})
	require.NoError(t, err)

	subscription, err := mutationResolver.PurchaseSubscriptionPlan(userCtx, biz.PurchaseSubscriptionPlanInput{
		UserID:    owner.ID,
		PlanID:    plan.ID,
		PromoCode: "plan10",
	})
	require.NoError(t, err)
	require.Equal(t, user.ID, subscription.UserID)
	require.Equal(t, usersubscription.StatusActive, subscription.Status)
	require.Equal(t, int64(50_000_000), subscription.OriginalPriceMicros)
	require.Equal(t, int64(5_000_000), subscription.DiscountAmountMicros)
	require.Equal(t, int64(45_000_000), subscription.PayableAmountMicros)

	account, err := queryResolver.MyBillingAccount(userCtx)
	require.NoError(t, err)
	require.Equal(t, int64(55_000_000), account.BalanceMicros)
	promoUsage, err := client.PromoUsage.Query().Where(promousage.UserSubscriptionIDEQ(subscription.ID)).Only(setupCtx)
	require.NoError(t, err)
	require.Equal(t, promousage.StatusApplied, promoUsage.Status)
	require.NotNil(t, promoUsage.UserID)
	require.Equal(t, user.ID, *promoUsage.UserID)
}

func checkoutParamsFromURL(t *testing.T, rawURL string) map[string]string {
	t.Helper()
	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	params := make(map[string]string, len(parsed.Query()))
	for key, values := range parsed.Query() {
		if len(values) > 0 {
			params[key] = values[0]
		}
	}
	return params
}
