package biz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
	"github.com/looplj/axonhub/internal/ent/usagehourlyaggregate"
)

type UsageAggregateServiceParams struct {
	fx.In

	Ent *ent.Client
}

type UsageAggregateService struct {
	*AbstractService
}

func NewUsageAggregateService(params UsageAggregateServiceParams) *UsageAggregateService {
	return &UsageAggregateService{AbstractService: &AbstractService{db: params.Ent}}
}

type UsageAggregateRebuildInput struct {
	From *time.Time
	To   *time.Time
}

type UsageAggregateRebuildResult struct {
	RecordsProcessed int `json:"recordsProcessed"`
	HourlyRows       int `json:"hourlyRows"`
	DailyRows        int `json:"dailyRows"`
}

type usageAggregatePoint struct {
	UserID             int
	APIKeyID           int
	ProjectID          int
	ChannelID          int
	UpstreamAccountID  int
	ModelID            string
	RequestType        string
	Status             string
	Currency           string
	RequestCount       int64
	SuccessCount       int64
	ErrorCount         int64
	PromptTokens       int64
	CompletionTokens   int64
	TotalTokens        int64
	UserChargeMicros   int64
	UpstreamCostMicros int64
	GrossMarginMicros  int64
	OccurredAt         time.Time
}

func (s *UsageAggregateService) ApplyBillingRecord(ctx context.Context, record *ent.UsageBillingRecord) error {
	if record == nil {
		return nil
	}

	return authz.RunWithSystemBypassVoid(ctx, "usage-aggregate-apply", func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		loaded, err := client.UsageBillingRecord.Query().
			Where(usagebillingrecord.IDEQ(record.ID)).
			WithUsageLog().
			Only(ctx)
		if err != nil {
			return fmt.Errorf("failed to load usage billing record %d for aggregation: %w", record.ID, err)
		}

		point := usageAggregatePointFromRecord(loaded)
		if err := s.upsertHourly(ctx, point); err != nil {
			return err
		}
		return s.upsertDaily(ctx, point)
	})
}

func (s *UsageAggregateService) Rebuild(ctx context.Context, input UsageAggregateRebuildInput) (*UsageAggregateRebuildResult, error) {
	return authz.RunWithSystemBypass(ctx, "usage-aggregate-rebuild", func(ctx context.Context) (*UsageAggregateRebuildResult, error) {
		client := s.entFromContext(ctx)
		if err := s.deleteAggregateRange(ctx, input); err != nil {
			return nil, err
		}

		query := client.UsageBillingRecord.Query().WithUsageLog()
		if input.From != nil {
			query.Where(usagebillingrecord.CreatedAtGTE(*input.From))
		}
		if input.To != nil {
			query.Where(usagebillingrecord.CreatedAtLTE(*input.To))
		}
		rows, err := query.Order(ent.Asc(usagebillingrecord.FieldID)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query usage billing records for aggregate rebuild: %w", err)
		}

		for _, row := range rows {
			point := usageAggregatePointFromRecord(row)
			if err := s.upsertHourly(ctx, point); err != nil {
				return nil, err
			}
			if err := s.upsertDaily(ctx, point); err != nil {
				return nil, err
			}
		}

		hourlyRows, err := client.UsageHourlyAggregate.Query().Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to count hourly usage aggregates: %w", err)
		}
		dailyRows, err := client.UsageDailyAggregate.Query().Count(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to count daily usage aggregates: %w", err)
		}

		return &UsageAggregateRebuildResult{
			RecordsProcessed: len(rows),
			HourlyRows:       hourlyRows,
			DailyRows:        dailyRows,
		}, nil
	})
}

func (s *UsageAggregateService) deleteAggregateRange(ctx context.Context, input UsageAggregateRebuildInput) error {
	client := s.entFromContext(ctx)

	hourlyDelete := client.UsageHourlyAggregate.Delete()
	dailyDelete := client.UsageDailyAggregate.Delete()
	if input.From != nil {
		hourlyDelete.Where(usagehourlyaggregate.BucketStartGTE(truncateUsageHour(*input.From)))
		dailyDelete.Where(usagedailyaggregate.BucketStartGTE(truncateUsageDay(*input.From)))
	}
	if input.To != nil {
		hourlyDelete.Where(usagehourlyaggregate.BucketStartLTE(truncateUsageHour(*input.To)))
		dailyDelete.Where(usagedailyaggregate.BucketStartLTE(truncateUsageDay(*input.To)))
	}

	if _, err := hourlyDelete.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete hourly usage aggregates: %w", err)
	}
	if _, err := dailyDelete.Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete daily usage aggregates: %w", err)
	}
	return nil
}

func (s *UsageAggregateService) upsertHourly(ctx context.Context, point usageAggregatePoint) error {
	bucket := truncateUsageHour(point.OccurredAt)
	err := s.entFromContext(ctx).UsageHourlyAggregate.Create().
		SetBucketStart(bucket).
		SetUserID(point.UserID).
		SetAPIKeyID(point.APIKeyID).
		SetProjectID(point.ProjectID).
		SetChannelID(point.ChannelID).
		SetUpstreamAccountID(point.UpstreamAccountID).
		SetModelID(point.ModelID).
		SetRequestType(usagehourlyaggregate.RequestType(point.RequestType)).
		SetStatus(usagehourlyaggregate.Status(point.Status)).
		SetCurrency(point.Currency).
		SetRequestCount(point.RequestCount).
		SetSuccessCount(point.SuccessCount).
		SetErrorCount(point.ErrorCount).
		SetPromptTokens(point.PromptTokens).
		SetCompletionTokens(point.CompletionTokens).
		SetTotalTokens(point.TotalTokens).
		SetUserChargeMicros(point.UserChargeMicros).
		SetUpstreamCostMicros(point.UpstreamCostMicros).
		SetGrossMarginMicros(point.GrossMarginMicros).
		OnConflictColumns(usageAggregateConflictColumns(usagehourlyaggregate.FieldBucketStart)...).
		Update(func(update *ent.UsageHourlyAggregateUpsert) {
			update.SetUpdatedAt(time.Now().UTC())
			update.AddRequestCount(point.RequestCount)
			update.AddSuccessCount(point.SuccessCount)
			update.AddErrorCount(point.ErrorCount)
			update.AddPromptTokens(point.PromptTokens)
			update.AddCompletionTokens(point.CompletionTokens)
			update.AddTotalTokens(point.TotalTokens)
			update.AddUserChargeMicros(point.UserChargeMicros)
			update.AddUpstreamCostMicros(point.UpstreamCostMicros)
			update.AddGrossMarginMicros(point.GrossMarginMicros)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert hourly usage aggregate: %w", err)
	}
	return nil
}

func (s *UsageAggregateService) upsertDaily(ctx context.Context, point usageAggregatePoint) error {
	bucket := truncateUsageDay(point.OccurredAt)
	err := s.entFromContext(ctx).UsageDailyAggregate.Create().
		SetBucketStart(bucket).
		SetUserID(point.UserID).
		SetAPIKeyID(point.APIKeyID).
		SetProjectID(point.ProjectID).
		SetChannelID(point.ChannelID).
		SetUpstreamAccountID(point.UpstreamAccountID).
		SetModelID(point.ModelID).
		SetRequestType(usagedailyaggregate.RequestType(point.RequestType)).
		SetStatus(usagedailyaggregate.Status(point.Status)).
		SetCurrency(point.Currency).
		SetRequestCount(point.RequestCount).
		SetSuccessCount(point.SuccessCount).
		SetErrorCount(point.ErrorCount).
		SetPromptTokens(point.PromptTokens).
		SetCompletionTokens(point.CompletionTokens).
		SetTotalTokens(point.TotalTokens).
		SetUserChargeMicros(point.UserChargeMicros).
		SetUpstreamCostMicros(point.UpstreamCostMicros).
		SetGrossMarginMicros(point.GrossMarginMicros).
		OnConflictColumns(usageAggregateConflictColumns(usagedailyaggregate.FieldBucketStart)...).
		Update(func(update *ent.UsageDailyAggregateUpsert) {
			update.SetUpdatedAt(time.Now().UTC())
			update.AddRequestCount(point.RequestCount)
			update.AddSuccessCount(point.SuccessCount)
			update.AddErrorCount(point.ErrorCount)
			update.AddPromptTokens(point.PromptTokens)
			update.AddCompletionTokens(point.CompletionTokens)
			update.AddTotalTokens(point.TotalTokens)
			update.AddUserChargeMicros(point.UserChargeMicros)
			update.AddUpstreamCostMicros(point.UpstreamCostMicros)
			update.AddGrossMarginMicros(point.GrossMarginMicros)
		}).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert daily usage aggregate: %w", err)
	}
	return nil
}

func usageAggregatePointFromRecord(record *ent.UsageBillingRecord) usageAggregatePoint {
	usageLog := record.Edges.UsageLog
	point := usageAggregatePoint{
		UserID:             record.UserID,
		APIKeyID:           record.APIKeyID,
		ProjectID:          record.ProjectID,
		ModelID:            record.ModelID,
		RequestType:        string(record.RequestType),
		Status:             string(record.Status),
		Currency:           record.Currency,
		RequestCount:       1,
		UpstreamCostMicros: record.CostAmountMicros,
		OccurredAt:         record.CreatedAt,
	}
	if usageLog != nil {
		point.ChannelID = usageLog.ChannelID
		point.PromptTokens = usageLog.PromptTokens
		point.CompletionTokens = usageLog.CompletionTokens
		point.TotalTokens = usageLog.TotalTokens
	}
	if usageAggregateSuccessStatus(record.Status) {
		point.SuccessCount = 1
	}
	if usageAggregateErrorStatus(record.Status) {
		point.ErrorCount = 1
	}
	if record.Status == usagebillingrecord.StatusCharged {
		point.UserChargeMicros = record.ChargeAmountMicros
	}
	point.GrossMarginMicros = point.UserChargeMicros - point.UpstreamCostMicros
	return point
}

func usageAggregateSuccessStatus(status usagebillingrecord.Status) bool {
	return slices.Contains([]usagebillingrecord.Status{
		usagebillingrecord.StatusCharged,
		usagebillingrecord.StatusSkipped,
	}, status)
}

func usageAggregateErrorStatus(status usagebillingrecord.Status) bool {
	return slices.Contains([]usagebillingrecord.Status{
		usagebillingrecord.StatusFailed,
	}, status)
}

func usageAggregateConflictColumns(bucketField string) []string {
	return []string{
		bucketField,
		"user_id",
		"api_key_id",
		"project_id",
		"channel_id",
		"model_id",
		"request_type",
		"status",
		"currency",
		"upstream_account_id",
	}
}

func truncateUsageHour(value time.Time) time.Time {
	return value.UTC().Truncate(time.Hour)
}

func truncateUsageDay(value time.Time) time.Time {
	utc := value.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
