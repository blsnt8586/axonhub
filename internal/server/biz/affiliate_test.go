package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/affiliaterebate"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
)

func TestAffiliatePaymentRebateIsIdempotentAndTransferCreditsWalletOnce(t *testing.T) {
	t.Parallel()

	client, ctx, paymentSvc := newPaymentTestService(t, "affiliate_payment_rebate")
	affiliateSvc := newAffiliateTestService(client)
	paymentSvc.affiliateService = affiliateSvc

	inviter := createPromoTestUser(t, client, ctx, "affiliate-inviter@example.com")
	invitee := createPromoTestUser(t, client, ctx, "affiliate-invitee@example.com")
	_, err := affiliateSvc.SaveSetting(ctx, SaveAffiliateSettingInput{
		Enabled:              true,
		DefaultRebateRateBps: 1000,
		FreezeDays:           7,
		Currency:             "CNY",
	})
	require.NoError(t, err)
	profile, err := affiliateSvc.GetOrCreateProfile(ctx, inviter.ID)
	require.NoError(t, err)
	_, err = affiliateSvc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: invitee.ID, InviteCode: profile.InviteCode})
	require.NoError(t, err)

	provider, err := paymentSvc.GetOrCreateSimulatedEPayProvider(ctx, "http://axon.local")
	require.NoError(t, err)
	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		BillingSubject:     UserBillingSubject(invitee.ID),
		ProviderInstanceID: &provider.ID,
		ProviderType:       provider.ProviderType,
		Amount:             decimal.RequireFromString("100"),
		Currency:           "CNY",
	})
	require.NoError(t, err)

	notify := NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret")
	paid, err := paymentSvc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)
	require.Equal(t, paymentorder.StatusPaid, paid.Status)
	_, err = paymentSvc.HandleEPayNotify(ctx, HandleEPayNotifyInput{Params: notify})
	require.NoError(t, err)

	rebates, err := client.AffiliateRebate.Query().Where(affiliaterebate.PaymentOrderIDEQ(paid.ID)).All(ctx)
	require.NoError(t, err)
	require.Len(t, rebates, 1)
	require.Equal(t, int64(10_000_000), rebates[0].AmountMicros)
	require.Equal(t, affiliaterebate.StatusFrozen, rebates[0].Status)

	_, err = affiliateSvc.TransferAvailable(ctx, TransferAffiliateRebatesInput{
		UserID: inviter.ID,
		Now:    rebates[0].FreezeUntil.Add(-1),
	})
	require.ErrorIs(t, err, ErrAffiliateRebateFrozen)

	result, err := affiliateSvc.TransferAvailable(ctx, TransferAffiliateRebatesInput{
		UserID: inviter.ID,
		Now:    rebates[0].FreezeUntil.Add(1),
	})
	require.NoError(t, err)
	require.Equal(t, 1, result.TransferredCount)
	require.Equal(t, int64(10_000_000), result.TransferredMicros)
	require.Len(t, result.LedgerTransactionIDs, 1)

	result, err = affiliateSvc.TransferAvailable(ctx, TransferAffiliateRebatesInput{
		UserID: inviter.ID,
		Now:    rebates[0].FreezeUntil.Add(2),
	})
	require.NoError(t, err)
	require.Zero(t, result.TransferredCount)

	account, err := affiliateSvc.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(inviter.ID))
	require.NoError(t, err)
	require.Equal(t, int64(10_000_000), account.BalanceMicros)
	tx, err := client.LedgerTransaction.Get(ctx, resultLedgerID(t, client, ctx, inviter.ID))
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.TypeAffiliateRebate, tx.Type)
}

func TestAffiliateSubscriptionRebateUsesProfileRateOverride(t *testing.T) {
	t.Parallel()

	client, ctx, _, inviteeAccount := newUsageBillingTestProcessor(t, "affiliate_subscription_rebate")
	affiliateSvc := newAffiliateTestService(client)
	subscriptionSvc := newSubscriptionTestService(client)
	subscriptionSvc.affiliateService = affiliateSvc
	inviter := createPromoTestUser(t, client, ctx, "affiliate-sub-inviter@example.com")
	_, err := affiliateSvc.SaveSetting(ctx, SaveAffiliateSettingInput{
		Enabled:              true,
		DefaultRebateRateBps: 500,
		FreezeDays:           0,
		Currency:             "CNY",
	})
	require.NoError(t, err)
	override := 2000
	_, err = affiliateSvc.SaveProfile(ctx, SaveAffiliateProfileInput{
		UserID:                inviter.ID,
		RebateRateOverrideBps: &override,
	})
	require.NoError(t, err)
	profile, err := affiliateSvc.GetOrCreateProfile(ctx, inviter.ID)
	require.NoError(t, err)
	_, err = affiliateSvc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: inviteeAccount.OwnerID, InviteCode: profile.InviteCode})
	require.NoError(t, err)
	_, err = subscriptionSvc.ledgerService.Credit(ctx, inviteeAccount.ID, decimal.RequireFromString("100"), ledgertransaction.TypePaymentRecharge, "affiliate-sub-credit")
	require.NoError(t, err)
	plan, err := subscriptionSvc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:           "Affiliate Pro",
		PeriodDays:     30,
		Price:          decimal.RequireFromString("50"),
		IncludedAmount: decimal.RequireFromString("100"),
		Currency:       "CNY",
	})
	require.NoError(t, err)

	sub, err := subscriptionSvc.PurchasePlan(ctx, PurchaseSubscriptionPlanInput{UserID: inviteeAccount.OwnerID, PlanID: plan.ID})
	require.NoError(t, err)
	rebate, err := client.AffiliateRebate.Query().Where(affiliaterebate.UserSubscriptionIDEQ(sub.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 2000, rebate.RateBps)
	require.Equal(t, int64(10_000_000), rebate.AmountMicros)
}

func TestAffiliateInviteRejectsSelfAndCircularBindings(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newAffiliateServiceWithContext(t, "affiliate_invite_rejects")
	a := createPromoTestUser(t, client, ctx, "affiliate-a@example.com")
	b := createPromoTestUser(t, client, ctx, "affiliate-b@example.com")
	aProfile, err := svc.GetOrCreateProfile(ctx, a.ID)
	require.NoError(t, err)
	bProfile, err := svc.GetOrCreateProfile(ctx, b.ID)
	require.NoError(t, err)

	_, err = svc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: a.ID, InviteCode: aProfile.InviteCode})
	require.ErrorIs(t, err, ErrAffiliateSelfInvite)

	_, err = svc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: b.ID, InviteCode: aProfile.InviteCode})
	require.NoError(t, err)
	_, err = svc.BindInvite(ctx, BindAffiliateInviteInput{InviteeUserID: a.ID, InviteCode: bProfile.InviteCode})
	require.ErrorIs(t, err, ErrAffiliateCircularInvite)
}

func newAffiliateServiceWithContext(t *testing.T, name string) (*ent.Client, context.Context, *AffiliateService) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	return client, ctx, newAffiliateTestService(client)
}

func newAffiliateTestService(client *ent.Client) *AffiliateService {
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	return NewAffiliateService(AffiliateServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
	})
}

func resultLedgerID(t *testing.T, client *ent.Client, ctx context.Context, userID int) int {
	t.Helper()
	rebate, err := client.AffiliateRebate.Query().
		Where(
			affiliaterebate.InviterUserIDEQ(userID),
			affiliaterebate.StatusEQ(affiliaterebate.StatusTransferred),
		).
		Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, rebate.LedgerTransactionID)
	return *rebate.LedgerTransactionID
}
