package biz

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
)

func TestSubscriptionServicePurchaseDebitsWalletAndCreatesSubscription(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "subscription_purchase")
	svc := newSubscriptionTestService(client)
	processor.subscriptionService = svc
	_, err := processor.ledgerService.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "subscription-purchase-credit")
	require.NoError(t, err)

	plan, err := svc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:           "Pro",
		PeriodDays:     30,
		Price:          decimal.RequireFromString("3.50"),
		IncludedAmount: decimal.RequireFromString("20"),
		Currency:       "CNY",
	})
	require.NoError(t, err)

	sub, err := svc.PurchasePlan(ctx, PurchaseSubscriptionPlanInput{
		UserID: account.OwnerID,
		PlanID: plan.ID,
		Now:    time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.Equal(t, usersubscription.StatusActive, sub.Status)
	require.Equal(t, plan.ID, sub.PlanID)
	require.Equal(t, int64(20_000_000), sub.IncludedAmountMicros)
	require.NotZero(t, sub.PurchaseLedgerTransactionID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(6_500_000), reloaded.BalanceMicros)

	tx, err := client.LedgerTransaction.Get(ctx, sub.PurchaseLedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.TypeSubscriptionDeduct, tx.Type)
	require.Equal(t, ledgertransaction.DirectionDebit, tx.Direction)
}

func TestSubscriptionCoverageSkipsWalletChargeAndRecordsUsage(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "subscription_coverage")
	svc := newSubscriptionTestService(client)
	processor.subscriptionService = svc
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)

	plan, err := svc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:              "Included",
		PeriodDays:        30,
		IncludedAmount:    decimal.RequireFromString("2"),
		SupportedModelIDs: []string{"gpt-test"},
		Currency:          "CNY",
	})
	require.NoError(t, err)
	sub, err := svc.AdminAssign(ctx, AdminAssignSubscriptionInput{
		UserID: account.OwnerID,
		PlanID: plan.ID,
	})
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, 1, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusSkipped, record.Status)
	require.Equal(t, sub.ID, record.UserSubscriptionID)
	require.Zero(t, record.LedgerTransactionID)
	require.Equal(t, int64(1_500_000), record.ChargeAmountMicros)

	reloadedSub, err := client.UserSubscription.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1_500_000), reloadedSub.UsedAmountMicros)
	reloadedAccount, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Zero(t, reloadedAccount.BalanceMicros)
}

func TestExpiredSubscriptionFallsBackToWalletCharge(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "subscription_expired_fallback")
	svc := newSubscriptionTestService(client)
	processor.subscriptionService = svc
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	_, err = processor.ledgerService.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "expired-subscription-wallet-credit")
	require.NoError(t, err)

	plan, err := svc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:           "Expired",
		PeriodDays:     30,
		IncludedAmount: decimal.RequireFromString("20"),
		Currency:       "CNY",
	})
	require.NoError(t, err)
	start := time.Now().UTC().Add(-48 * time.Hour)
	expire := time.Now().UTC().Add(-24 * time.Hour)
	sub, err := svc.AdminAssign(ctx, AdminAssignSubscriptionInput{
		UserID:    account.OwnerID,
		PlanID:    plan.ID,
		StartsAt:  &start,
		ExpiresAt: &expire,
	})
	require.NoError(t, err)
	_, err = client.UserSubscription.UpdateOneID(sub.ID).SetStatus(usersubscription.StatusExpired).Save(ctx)
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, 1, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Zero(t, record.UserSubscriptionID)
	require.NotZero(t, record.LedgerTransactionID)

	reloadedAccount, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(8_500_000), reloadedAccount.BalanceMicros)
}

func TestSubscriptionUsageResetAndExpiryWorkers(t *testing.T) {
	t.Parallel()

	client, ctx, _, account := newUsageBillingTestProcessor(t, "subscription_reset_expiry")
	svc := newSubscriptionTestService(client)
	plan, err := svc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:           "Resettable",
		PeriodDays:     7,
		IncludedAmount: decimal.RequireFromString("5"),
		Currency:       "CNY",
	})
	require.NoError(t, err)
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	expire := start.AddDate(0, 0, 14)
	sub, err := svc.AdminAssign(ctx, AdminAssignSubscriptionInput{
		UserID:    account.OwnerID,
		PlanID:    plan.ID,
		StartsAt:  &start,
		ExpiresAt: &expire,
	})
	require.NoError(t, err)
	_, err = client.UserSubscription.UpdateOneID(sub.ID).
		SetUsedAmountMicros(4_000_000).
		SetCurrentPeriodEnd(start.AddDate(0, 0, 7)).
		SetResetAt(start.AddDate(0, 0, 7)).
		Save(ctx)
	require.NoError(t, err)

	reset, err := svc.ResetDueUsageWindows(ctx, start.AddDate(0, 0, 8), 10)
	require.NoError(t, err)
	require.Equal(t, 1, reset)
	reloaded, err := client.UserSubscription.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Zero(t, reloaded.UsedAmountMicros)
	require.Equal(t, start.AddDate(0, 0, 14), reloaded.ResetAt)

	expired, err := svc.ExpireDue(ctx, expire.Add(time.Minute), 10)
	require.NoError(t, err)
	require.Equal(t, 1, expired)
	reloaded, err = client.UserSubscription.Get(ctx, sub.ID)
	require.NoError(t, err)
	require.Equal(t, usersubscription.StatusExpired, reloaded.Status)
}

func newSubscriptionTestService(client *ent.Client) *SubscriptionService {
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	return NewSubscriptionService(SubscriptionServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
	})
}
