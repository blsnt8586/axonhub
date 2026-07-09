package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/upstreamaccount"
	"github.com/looplj/axonhub/internal/ent/upstreamaccountswitchhistory"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
)

type UpstreamAccountMonitoringFilter struct {
	From         *time.Time
	To           *time.Time
	ChannelID    *int
	ProviderType string
	Status       string
	ModelID      string
	AccountID    *int
}

type UpstreamAccountMonitoringSummary struct {
	AccountID           int
	AccountName         string
	ChannelID           int
	ChannelName         string
	ProviderType        string
	Status              string
	Schedulable         bool
	RequestCount        int
	SuccessCount        int
	ErrorCount          int
	SuccessRate         float64
	ErrorRate           float64
	RateLimitCount      int
	ServerErrorCount    int
	AverageLatencyMs    *float64
	FirstTokenLatencyMs *float64
	UserChargeMicros    int64
	UpstreamCostMicros  int64
	GrossMarginMicros   int64
	QuotaLimitMicros    int64
	QuotaUsedMicros     int64
	LastUsedAt          *time.Time
	LastErrorMessage    *string
	RateLimitResetAt    *time.Time
	OverloadUntil       *time.Time
	CooldownUntil       *time.Time
	CooldownReason      *string
	RecentErrors        []*UpstreamAccountRecentError
}

type UpstreamAccountRecentError struct {
	RequestExecutionID int
	RequestID          int
	StatusCode         *int
	ErrorMessage       string
	CreatedAt          time.Time
}

type UpstreamAccountUsageTrendPoint struct {
	BucketStart        time.Time
	RequestCount       int64
	SuccessCount       int64
	ErrorCount         int64
	UserChargeMicros   int64
	UpstreamCostMicros int64
	GrossMarginMicros  int64
	TotalTokens        int64
}

type UpstreamAccountMonitoringDetail struct {
	Summary          *UpstreamAccountMonitoringSummary
	UsageTrend       []*UpstreamAccountUsageTrendPoint
	RecentExecutions []*ent.RequestExecution
	SwitchHistory    []*ent.UpstreamAccountSwitchHistory
}

func (s *UpstreamAccountService) MonitoringSummaries(ctx context.Context, filter UpstreamAccountMonitoringFilter) ([]*UpstreamAccountMonitoringSummary, error) {
	accounts, err := s.monitoringAccounts(ctx, filter)
	if err != nil {
		return nil, err
	}

	summaries := make([]*UpstreamAccountMonitoringSummary, 0, len(accounts))
	for _, account := range accounts {
		summary, err := s.monitoringSummaryForAccount(ctx, account, filter)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		if summaries[i].RequestCount != summaries[j].RequestCount {
			return summaries[i].RequestCount > summaries[j].RequestCount
		}
		if summaries[i].ErrorRate != summaries[j].ErrorRate {
			return summaries[i].ErrorRate > summaries[j].ErrorRate
		}
		return summaries[i].AccountID < summaries[j].AccountID
	})

	return summaries, nil
}

func (s *UpstreamAccountService) MonitoringDetail(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) (*UpstreamAccountMonitoringDetail, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("account id is required")
	}
	filter.AccountID = &accountID

	account, err := s.entFromContext(ctx).UpstreamAccount.Query().
		Where(upstreamaccount.IDEQ(accountID)).
		WithChannel().
		Only(ctx)
	if err != nil {
		return nil, err
	}

	summary, err := s.monitoringSummaryForAccount(ctx, account, filter)
	if err != nil {
		return nil, err
	}
	trend, err := s.monitoringUsageTrend(ctx, accountID, filter)
	if err != nil {
		return nil, err
	}
	executions, err := s.monitoringRecentExecutions(ctx, accountID, filter)
	if err != nil {
		return nil, err
	}
	switchHistory, err := s.monitoringSwitchHistory(ctx, accountID, filter)
	if err != nil {
		return nil, err
	}

	return &UpstreamAccountMonitoringDetail{
		Summary:          summary,
		UsageTrend:       trend,
		RecentExecutions: executions,
		SwitchHistory:    switchHistory,
	}, nil
}

func (s *UpstreamAccountService) monitoringAccounts(ctx context.Context, filter UpstreamAccountMonitoringFilter) ([]*ent.UpstreamAccount, error) {
	query := s.entFromContext(ctx).UpstreamAccount.Query().
		WithChannel().
		Where(upstreamaccount.StatusNEQ(upstreamaccount.StatusArchived))
	if filter.AccountID != nil && *filter.AccountID > 0 {
		query.Where(upstreamaccount.IDEQ(*filter.AccountID))
	}
	if filter.ChannelID != nil && *filter.ChannelID > 0 {
		query.Where(upstreamaccount.ChannelIDEQ(*filter.ChannelID))
	}
	if status := strings.TrimSpace(filter.Status); status != "" && status != "all" {
		query.Where(upstreamaccount.StatusEQ(upstreamaccount.Status(status)))
	}
	if providerType := strings.TrimSpace(filter.ProviderType); providerType != "" && providerType != "all" {
		query.Where(upstreamaccount.HasChannelWith(channel.TypeEQ(channel.Type(providerType))))
	}

	return query.Order(ent.Asc(upstreamaccount.FieldChannelID), ent.Asc(upstreamaccount.FieldPriority), ent.Asc(upstreamaccount.FieldID)).All(ctx)
}

func (s *UpstreamAccountService) monitoringSummaryForAccount(ctx context.Context, account *ent.UpstreamAccount, filter UpstreamAccountMonitoringFilter) (*UpstreamAccountMonitoringSummary, error) {
	execPreds := s.executionPredicatesForAccount(account.ID, filter)

	requestCount, err := s.entFromContext(ctx).RequestExecution.Query().Where(execPreds...).Count(ctx)
	if err != nil {
		return nil, err
	}
	successCount, err := s.entFromContext(ctx).RequestExecution.Query().Where(append(execPreds, requestexecution.StatusEQ(requestexecution.StatusCompleted))...).Count(ctx)
	if err != nil {
		return nil, err
	}
	errorCount, err := s.entFromContext(ctx).RequestExecution.Query().Where(append(execPreds, requestexecution.StatusEQ(requestexecution.StatusFailed))...).Count(ctx)
	if err != nil {
		return nil, err
	}
	rateLimitCount, err := s.entFromContext(ctx).RequestExecution.Query().Where(append(execPreds, requestexecution.ResponseStatusCodeEQ(429))...).Count(ctx)
	if err != nil {
		return nil, err
	}
	serverErrorCount, err := s.entFromContext(ctx).RequestExecution.Query().Where(append(execPreds, requestexecution.ResponseStatusCodeGTE(500), requestexecution.ResponseStatusCodeLTE(599))...).Count(ctx)
	if err != nil {
		return nil, err
	}

	avgLatency, avgFirstToken, err := s.monitoringLatencyAverages(ctx, execPreds)
	if err != nil {
		return nil, err
	}
	userCharge, upstreamCost, grossMargin, err := s.monitoringBillingTotals(ctx, account.ID, filter)
	if err != nil {
		return nil, err
	}
	recentErrors, err := s.monitoringRecentErrors(ctx, account.ID, filter)
	if err != nil {
		return nil, err
	}

	channelName := ""
	providerType := ""
	if account.Edges.Channel != nil {
		channelName = account.Edges.Channel.Name
		providerType = string(account.Edges.Channel.Type)
	}

	summary := &UpstreamAccountMonitoringSummary{
		AccountID:           account.ID,
		AccountName:         account.Name,
		ChannelID:           account.ChannelID,
		ChannelName:         channelName,
		ProviderType:        providerType,
		Status:              string(account.Status),
		Schedulable:         account.Schedulable,
		RequestCount:        requestCount,
		SuccessCount:        successCount,
		ErrorCount:          errorCount,
		RateLimitCount:      rateLimitCount,
		ServerErrorCount:    serverErrorCount,
		AverageLatencyMs:    avgLatency,
		FirstTokenLatencyMs: avgFirstToken,
		UserChargeMicros:    userCharge,
		UpstreamCostMicros:  upstreamCost,
		GrossMarginMicros:   grossMargin,
		QuotaLimitMicros:    account.QuotaLimitMicros,
		QuotaUsedMicros:     account.QuotaUsedMicros,
		LastUsedAt:          account.LastUsedAt,
		LastErrorMessage:    account.ErrorMessage,
		RateLimitResetAt:    account.RateLimitResetAt,
		OverloadUntil:       account.OverloadUntil,
		CooldownUntil:       account.CooldownUntil,
		CooldownReason:      account.CooldownReason,
		RecentErrors:        recentErrors,
	}
	if requestCount > 0 {
		summary.SuccessRate = float64(successCount) / float64(requestCount)
		summary.ErrorRate = float64(errorCount) / float64(requestCount)
	}

	return summary, nil
}

func (s *UpstreamAccountService) executionPredicatesForAccount(accountID int, filter UpstreamAccountMonitoringFilter) []predicate.RequestExecution {
	preds := []predicate.RequestExecution{requestexecution.UpstreamAccountIDEQ(accountID)}
	if filter.From != nil {
		preds = append(preds, requestexecution.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		preds = append(preds, requestexecution.CreatedAtLTE(*filter.To))
	}
	if filter.ChannelID != nil && *filter.ChannelID > 0 {
		preds = append(preds, requestexecution.ChannelIDEQ(*filter.ChannelID))
	}
	if modelID := strings.TrimSpace(filter.ModelID); modelID != "" {
		preds = append(preds, requestexecution.ModelIDEQ(modelID))
	}
	return preds
}

func (s *UpstreamAccountService) billingPredicatesForAccount(accountID int, filter UpstreamAccountMonitoringFilter) []predicate.UsageBillingRecord {
	preds := []predicate.UsageBillingRecord{usagebillingrecord.UpstreamAccountIDEQ(accountID)}
	if filter.From != nil {
		preds = append(preds, usagebillingrecord.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		preds = append(preds, usagebillingrecord.CreatedAtLTE(*filter.To))
	}
	if modelID := strings.TrimSpace(filter.ModelID); modelID != "" {
		preds = append(preds, usagebillingrecord.ModelIDEQ(modelID))
	}
	return preds
}

func (s *UpstreamAccountService) monitoringLatencyAverages(ctx context.Context, preds []predicate.RequestExecution) (*float64, *float64, error) {
	var rows []struct {
		AverageLatencyMs    *float64 `json:"average_latency_ms"`
		FirstTokenLatencyMs *float64 `json:"first_token_latency_ms"`
	}
	err := s.entFromContext(ctx).RequestExecution.Query().
		Where(preds...).
		Modify(func(selector *sql.Selector) {
			selector.Select(
				sql.As(sql.Avg(selector.C(requestexecution.FieldMetricsLatencyMs)), "average_latency_ms"),
				sql.As(sql.Avg(selector.C(requestexecution.FieldMetricsFirstTokenLatencyMs)), "first_token_latency_ms"),
			)
		}).
		Scan(ctx, &rows)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) == 0 {
		return nil, nil, nil
	}

	return rows[0].AverageLatencyMs, rows[0].FirstTokenLatencyMs, nil
}

func (s *UpstreamAccountService) monitoringBillingTotals(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) (int64, int64, int64, error) {
	var rows []struct {
		UserChargeMicros   *int64 `json:"user_charge_micros"`
		UpstreamCostMicros *int64 `json:"upstream_cost_micros"`
	}
	err := s.entFromContext(ctx).UsageBillingRecord.Query().
		Where(s.billingPredicatesForAccount(accountID, filter)...).
		Modify(func(selector *sql.Selector) {
			selector.Select(
				sql.As(sql.Sum(selector.C(usagebillingrecord.FieldChargeAmountMicros)), "user_charge_micros"),
				sql.As(sql.Sum(selector.C(usagebillingrecord.FieldCostAmountMicros)), "upstream_cost_micros"),
			)
		}).
		Scan(ctx, &rows)
	if err != nil {
		return 0, 0, 0, err
	}
	if len(rows) == 0 {
		return 0, 0, 0, nil
	}

	var userCharge int64
	if rows[0].UserChargeMicros != nil {
		userCharge = *rows[0].UserChargeMicros
	}
	var upstreamCost int64
	if rows[0].UpstreamCostMicros != nil {
		upstreamCost = *rows[0].UpstreamCostMicros
	}

	return userCharge, upstreamCost, userCharge - upstreamCost, nil
}

func (s *UpstreamAccountService) monitoringRecentErrors(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) ([]*UpstreamAccountRecentError, error) {
	preds := append(s.executionPredicatesForAccount(accountID, filter), requestexecution.StatusEQ(requestexecution.StatusFailed))
	rows, err := s.entFromContext(ctx).RequestExecution.Query().
		Where(preds...).
		Order(ent.Desc(requestexecution.FieldCreatedAt)).
		Limit(5).
		All(ctx)
	if err != nil {
		return nil, err
	}

	result := make([]*UpstreamAccountRecentError, 0, len(rows))
	for _, row := range rows {
		result = append(result, &UpstreamAccountRecentError{
			RequestExecutionID: row.ID,
			RequestID:          row.RequestID,
			StatusCode:         row.ResponseStatusCode,
			ErrorMessage:       row.ErrorMessage,
			CreatedAt:          row.CreatedAt,
		})
	}
	return result, nil
}

func (s *UpstreamAccountService) monitoringUsageTrend(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) ([]*UpstreamAccountUsageTrendPoint, error) {
	query := s.entFromContext(ctx).UsageDailyAggregate.Query().
		Where(usagedailyaggregate.UpstreamAccountIDEQ(accountID))
	if filter.From != nil {
		query.Where(usagedailyaggregate.BucketStartGTE(truncateUsageDay(*filter.From)))
	}
	if filter.To != nil {
		query.Where(usagedailyaggregate.BucketStartLTE(truncateUsageDay(*filter.To)))
	}
	if filter.ChannelID != nil && *filter.ChannelID > 0 {
		query.Where(usagedailyaggregate.ChannelIDEQ(*filter.ChannelID))
	}
	if modelID := strings.TrimSpace(filter.ModelID); modelID != "" {
		query.Where(usagedailyaggregate.ModelIDEQ(modelID))
	}

	rows, err := query.Order(ent.Asc(usagedailyaggregate.FieldBucketStart)).All(ctx)
	if err != nil {
		return nil, err
	}

	byBucket := make(map[time.Time]*UpstreamAccountUsageTrendPoint)
	for _, row := range rows {
		point := byBucket[row.BucketStart]
		if point == nil {
			point = &UpstreamAccountUsageTrendPoint{BucketStart: row.BucketStart}
			byBucket[row.BucketStart] = point
		}
		point.RequestCount += row.RequestCount
		point.SuccessCount += row.SuccessCount
		point.ErrorCount += row.ErrorCount
		point.UserChargeMicros += row.UserChargeMicros
		point.UpstreamCostMicros += row.UpstreamCostMicros
		point.GrossMarginMicros += row.GrossMarginMicros
		point.TotalTokens += row.TotalTokens
	}

	result := make([]*UpstreamAccountUsageTrendPoint, 0, len(byBucket))
	for _, point := range byBucket {
		result = append(result, point)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].BucketStart.Before(result[j].BucketStart)
	})

	return result, nil
}

func (s *UpstreamAccountService) monitoringRecentExecutions(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) ([]*ent.RequestExecution, error) {
	return s.entFromContext(ctx).RequestExecution.Query().
		Where(s.executionPredicatesForAccount(accountID, filter)...).
		Order(ent.Desc(requestexecution.FieldCreatedAt)).
		Limit(20).
		All(ctx)
}

func (s *UpstreamAccountService) monitoringSwitchHistory(ctx context.Context, accountID int, filter UpstreamAccountMonitoringFilter) ([]*ent.UpstreamAccountSwitchHistory, error) {
	query := s.entFromContext(ctx).UpstreamAccountSwitchHistory.Query().
		Where(upstreamaccountswitchhistory.Or(
			upstreamaccountswitchhistory.FromAccountIDEQ(accountID),
			upstreamaccountswitchhistory.ToAccountIDEQ(accountID),
		)).
		WithFromAccount().
		WithToAccount().
		WithChannel().
		Order(ent.Desc(upstreamaccountswitchhistory.FieldCreatedAt)).
		Limit(50)
	if filter.From != nil {
		query.Where(upstreamaccountswitchhistory.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		query.Where(upstreamaccountswitchhistory.CreatedAtLTE(*filter.To))
	}
	if filter.ChannelID != nil && *filter.ChannelID > 0 {
		query.Where(upstreamaccountswitchhistory.ChannelIDEQ(*filter.ChannelID))
	}
	if modelID := strings.TrimSpace(filter.ModelID); modelID != "" {
		query.Where(upstreamaccountswitchhistory.ModelIDEQ(modelID))
	}

	return query.All(ctx)
}
