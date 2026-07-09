package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
	"github.com/looplj/axonhub/internal/ent/usagehourlyaggregate"
)

func TestUsageAggregateServiceRebuildIsRepeatableAndMatchesDetailRows(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_aggregate_rebuild")
	createUsageAggregatePriceRule(t, client, ctx)
	creditUsageAggregateAccount(t, client, ctx, account.ID, "10")

	firstLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-aggregate", 1_000_000, 500_000)
	first, err := processor.BillUsage(ctx, firstLog.ID)
	require.NoError(t, err)
	secondLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "sora-aggregate", 2_000_000, 0)
	second, err := processor.BillUsage(ctx, secondLog.ID)
	require.NoError(t, err)

	aggregateSvc := NewUsageAggregateService(UsageAggregateServiceParams{Ent: client})
	firstRebuild, err := aggregateSvc.Rebuild(ctx, UsageAggregateRebuildInput{})
	require.NoError(t, err)
	require.Equal(t, 2, firstRebuild.RecordsProcessed)

	expectedCharge := first.ChargeAmountMicros + second.ChargeAmountMicros
	dailyTotals := usageDailyAggregateTotals(t, client, ctx)
	require.Equal(t, int64(2), dailyTotals.requestCount)
	require.Equal(t, int64(2), dailyTotals.successCount)
	require.Equal(t, expectedCharge, dailyTotals.userChargeMicros)
	require.Equal(t, int64(3_000_000), dailyTotals.promptTokens)
	require.Equal(t, int64(500_000), dailyTotals.completionTokens)
	require.Equal(t, int64(3_500_000), dailyTotals.totalTokens)

	secondRebuild, err := aggregateSvc.Rebuild(ctx, UsageAggregateRebuildInput{})
	require.NoError(t, err)
	require.Equal(t, firstRebuild.RecordsProcessed, secondRebuild.RecordsProcessed)
	require.Equal(t, dailyTotals, usageDailyAggregateTotals(t, client, ctx))
}

func TestUsageBillingProcessorDoesNotDoubleCountUsageAggregates(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_aggregate_duplicate_billing")
	createUsageAggregatePriceRule(t, client, ctx)
	creditUsageAggregateAccount(t, client, ctx, account.ID, "10")

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-aggregate", 1_000_000, 0)
	first, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	second, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	dailyTotals := usageDailyAggregateTotals(t, client, ctx)
	require.Equal(t, int64(1), dailyTotals.requestCount)
	require.Equal(t, first.ChargeAmountMicros, dailyTotals.userChargeMicros)

	hourlyTotals := usageHourlyAggregateTotals(t, client, ctx)
	require.Equal(t, int64(1), hourlyTotals.requestCount)
	require.Equal(t, first.ChargeAmountMicros, hourlyTotals.userChargeMicros)
}

type usageAggregateTotals struct {
	requestCount       int64
	successCount       int64
	errorCount         int64
	promptTokens       int64
	completionTokens   int64
	totalTokens        int64
	userChargeMicros   int64
	upstreamCostMicros int64
	grossMarginMicros  int64
}

func usageDailyAggregateTotals(t *testing.T, client *ent.Client, ctx context.Context) usageAggregateTotals {
	t.Helper()

	rows, err := client.UsageDailyAggregate.Query().
		Where(usagedailyaggregate.StatusEQ(usagedailyaggregate.StatusCharged)).
		All(ctx)
	require.NoError(t, err)

	var totals usageAggregateTotals
	for _, row := range rows {
		totals.requestCount += row.RequestCount
		totals.successCount += row.SuccessCount
		totals.errorCount += row.ErrorCount
		totals.promptTokens += row.PromptTokens
		totals.completionTokens += row.CompletionTokens
		totals.totalTokens += row.TotalTokens
		totals.userChargeMicros += row.UserChargeMicros
		totals.upstreamCostMicros += row.UpstreamCostMicros
		totals.grossMarginMicros += row.GrossMarginMicros
	}
	return totals
}

func usageHourlyAggregateTotals(t *testing.T, client *ent.Client, ctx context.Context) usageAggregateTotals {
	t.Helper()

	rows, err := client.UsageHourlyAggregate.Query().
		Where(usagehourlyaggregate.StatusEQ(usagehourlyaggregate.StatusCharged)).
		All(ctx)
	require.NoError(t, err)

	var totals usageAggregateTotals
	for _, row := range rows {
		totals.requestCount += row.RequestCount
		totals.successCount += row.SuccessCount
		totals.errorCount += row.ErrorCount
		totals.promptTokens += row.PromptTokens
		totals.completionTokens += row.CompletionTokens
		totals.totalTokens += row.TotalTokens
		totals.userChargeMicros += row.UserChargeMicros
		totals.upstreamCostMicros += row.UpstreamCostMicros
		totals.grossMarginMicros += row.GrossMarginMicros
	}
	return totals
}

func createUsageAggregatePriceRule(t *testing.T, client *ent.Client, ctx context.Context) {
	t.Helper()

	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("*").
		SetPrice(testModelPrice("1")).
		SetReferenceID("aggregate-sell-v1").
		Save(ctx)
	require.NoError(t, err)
}

func creditUsageAggregateAccount(t *testing.T, client *ent.Client, ctx context.Context, accountID int, amount string) {
	t.Helper()

	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err := ledgerSvc.Credit(ctx, accountID, decimal.RequireFromString(amount), ledgertransaction.TypePaymentRecharge, "aggregate-credit")
	require.NoError(t, err)
}
