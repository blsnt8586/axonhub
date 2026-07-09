package biz

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/upstreamaccount"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
	"github.com/looplj/axonhub/internal/ent/usagehourlyaggregate"
	"github.com/looplj/axonhub/internal/objects"
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

func TestUsageBillingProcessorPropagatesUpstreamAccountToBillingAndAggregates(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_aggregate_upstream_account")
	createUsageAggregatePriceRule(t, client, ctx)
	creditUsageAggregateAccount(t, client, ctx, account.ID, "10")

	channelRow, upstreamAccount := createUsageAggregateUpstreamAccount(t, client, ctx)
	usageLog := createUsageLogForBillingTestWithUpstreamAccount(t, client, ctx, account.OwnerID, channelRow.ID, upstreamAccount.ID, "gpt-aggregate", 1_000_000, 0)

	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.NotNil(t, record.UpstreamAccountID)
	require.Equal(t, upstreamAccount.ID, *record.UpstreamAccountID)

	hourlyRows, err := client.UsageHourlyAggregate.Query().
		Where(usagehourlyaggregate.UpstreamAccountIDEQ(upstreamAccount.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, hourlyRows, 1)
	require.Equal(t, int64(1), hourlyRows[0].RequestCount)
	require.Equal(t, record.ChargeAmountMicros, hourlyRows[0].UserChargeMicros)

	dailyRows, err := client.UsageDailyAggregate.Query().
		Where(usagedailyaggregate.UpstreamAccountIDEQ(upstreamAccount.ID)).
		All(ctx)
	require.NoError(t, err)
	require.Len(t, dailyRows, 1)
	require.Equal(t, int64(1), dailyRows[0].RequestCount)
	require.Equal(t, record.ChargeAmountMicros, dailyRows[0].UserChargeMicros)
}

func TestUpstreamAccountMonitoringSummariesAndDetail(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "upstream_account_monitoring")
	createUsageAggregatePriceRule(t, client, ctx)
	creditUsageAggregateAccount(t, client, ctx, account.ID, "10")
	channelRow, upstreamAccount := createUsageAggregateUpstreamAccount(t, client, ctx)
	now := time.Now().UTC()

	createMonitoringExecution(t, client, ctx, account.OwnerID, channelRow.ID, upstreamAccount.ID, requestexecution.StatusCompleted, nil, 120, 30, now.Add(-3*time.Minute))
	createMonitoringExecution(t, client, ctx, account.OwnerID, channelRow.ID, upstreamAccount.ID, requestexecution.StatusCompleted, nil, 80, 20, now.Add(-2*time.Minute))
	rateLimited := 429
	createMonitoringExecution(t, client, ctx, account.OwnerID, channelRow.ID, upstreamAccount.ID, requestexecution.StatusFailed, &rateLimited, 60, 0, now.Add(-time.Minute))

	usageLog := createUsageLogForBillingTestWithUpstreamAccount(t, client, ctx, account.OwnerID, channelRow.ID, upstreamAccount.ID, "gpt-aggregate", 1_000_000, 0)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)

	svc := NewUpstreamAccountService(UpstreamAccountServiceParams{Ent: client})
	fromAccountID := upstreamAccount.ID
	toAccountID := upstreamAccount.ID
	err = svc.RecordSwitchHistory(ctx, UpstreamAccountSwitchHistoryInput{
		ProjectID:     account.OwnerID,
		ChannelID:     channelRow.ID,
		FromAccountID: &fromAccountID,
		ToAccountID:   &toAccountID,
		ModelID:       "gpt-aggregate",
		Reason:        "retry_after_failure",
		ErrorCode:     &rateLimited,
		ErrorMessage:  "rate limited",
		LatencyMs:     pointerInt64(60),
	})
	require.NoError(t, err)

	summaries, err := svc.MonitoringSummaries(ctx, UpstreamAccountMonitoringFilter{AccountID: &upstreamAccount.ID})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	summary := summaries[0]
	require.Equal(t, 3, summary.RequestCount)
	require.Equal(t, 2, summary.SuccessCount)
	require.Equal(t, 1, summary.ErrorCount)
	require.Equal(t, 1, summary.RateLimitCount)
	require.Equal(t, 0, summary.ServerErrorCount)
	require.InDelta(t, 2.0/3.0, summary.SuccessRate, 0.0001)
	require.NotNil(t, summary.AverageLatencyMs)
	require.InDelta(t, 260.0/3.0, *summary.AverageLatencyMs, 0.0001)
	require.Equal(t, record.ChargeAmountMicros, summary.UserChargeMicros)
	require.Len(t, summary.RecentErrors, 1)
	require.Equal(t, rateLimited, *summary.RecentErrors[0].StatusCode)

	detail, err := svc.MonitoringDetail(ctx, upstreamAccount.ID, UpstreamAccountMonitoringFilter{})
	require.NoError(t, err)
	require.NotNil(t, detail.Summary)
	require.NotEmpty(t, detail.UsageTrend)
	require.Len(t, detail.RecentExecutions, 3)
	require.Len(t, detail.SwitchHistory, 1)
	require.Equal(t, "retry_after_failure", detail.SwitchHistory[0].Reason)
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

func createUsageAggregateUpstreamAccount(t *testing.T, client *ent.Client, ctx context.Context) (*ent.Channel, *ent.UpstreamAccount) {
	t.Helper()

	channelRow, err := client.Channel.Create().
		SetName("account-monitor-channel").
		SetType("openai").
		SetBaseURL("https://example.test/v1").
		SetSupportedModels([]string{"gpt-aggregate"}).
		SetDefaultTestModel("gpt-aggregate").
		SetCredentials(objects.ChannelCredentials{APIKey: "channel-key"}).
		Save(ctx)
	require.NoError(t, err)

	accountRow, err := client.UpstreamAccount.Create().
		SetChannelID(channelRow.ID).
		SetName("account-a").
		SetCredentialType(upstreamaccount.CredentialTypeAPIKey).
		SetCredentials(objects.UpstreamAccountCredentials{APIKey: "account-key"}).
		SetStatus(upstreamaccount.StatusActive).
		SetSchedulable(true).
		SetWeight(100).
		Save(ctx)
	require.NoError(t, err)

	return channelRow, accountRow
}

func createUsageLogForBillingTestWithUpstreamAccount(
	t *testing.T,
	client *ent.Client,
	ctx context.Context,
	projectID int,
	channelID int,
	upstreamAccountID int,
	modelID string,
	promptTokens int64,
	completionTokens int64,
) *ent.UsageLog {
	t.Helper()

	apiKey, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(projectID)).
		First(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetAPIKeyID(apiKey.ID).
		SetProjectID(projectID).
		SetChannelID(channelID).
		SetModelID(modelID).
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(apiKey.ID).
		SetProjectID(projectID).
		SetChannelID(channelID).
		SetUpstreamAccountID(upstreamAccountID).
		SetModelID(modelID).
		SetPromptTokens(promptTokens).
		SetCompletionTokens(completionTokens).
		SetTotalTokens(promptTokens + completionTokens).
		SetTotalCost(0.25).
		Save(ctx)
	require.NoError(t, err)

	return usageLog
}

func createMonitoringExecution(
	t *testing.T,
	client *ent.Client,
	ctx context.Context,
	projectID int,
	channelID int,
	upstreamAccountID int,
	status requestexecution.Status,
	statusCode *int,
	latencyMs int64,
	firstTokenMs int64,
	createdAt time.Time,
) *ent.RequestExecution {
	t.Helper()

	req, err := client.Request.Create().
		SetProjectID(projectID).
		SetChannelID(channelID).
		SetModelID("gpt-aggregate").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	create := client.RequestExecution.Create().
		SetProjectID(projectID).
		SetRequestID(req.ID).
		SetChannelID(channelID).
		SetUpstreamAccountID(upstreamAccountID).
		SetModelID("gpt-aggregate").
		SetFormat("openai/chat_completions").
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetStatus(status).
		SetStream(false).
		SetMetricsLatencyMs(latencyMs).
		SetMetricsFirstTokenLatencyMs(firstTokenMs).
		SetCreatedAt(createdAt).
		SetUpdatedAt(createdAt)
	if statusCode != nil {
		create.SetResponseStatusCode(*statusCode)
	}
	if status == requestexecution.StatusFailed {
		create.SetErrorMessage("request failed")
	}
	row, err := create.Save(ctx)
	require.NoError(t, err)
	return row
}

func pointerInt64(value int64) *int64 {
	return &value
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
