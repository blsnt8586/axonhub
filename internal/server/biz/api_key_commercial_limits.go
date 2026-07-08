package biz

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"entgo.io/ent/dialect/sql"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xtime"
)

var ErrAPIKeyCommercialLimitExceeded = errors.New("api key commercial limit exceeded")

type APIKeyCommercialLimitServiceParams struct {
	fx.In

	Ent           *ent.Client
	SystemService *SystemService
}

type APIKeyCommercialLimitService struct {
	ent    *ent.Client
	system *SystemService
}

func NewAPIKeyCommercialLimitService(params APIKeyCommercialLimitServiceParams) *APIKeyCommercialLimitService {
	return &APIKeyCommercialLimitService{
		ent:    params.Ent,
		system: params.SystemService,
	}
}

type APIKeyCommercialLimitCheckInput struct {
	APIKey                *ent.APIKey
	Currency              string
	EstimatedChargeMicros int64
	Now                   time.Time
}

type APIKeyCommercialLimitCheckResult struct {
	Allowed bool
	Message string
	Usage   CommercialLimitUsageResult
}

type CommercialLimitUsageResult struct {
	APIKeyID               objects.GUID
	Currency               string
	Enabled                bool
	Total                  CommercialLimitWindowUsageResult
	Daily                  CommercialLimitWindowUsageResult
	Monthly                CommercialLimitWindowUsageResult
	SingleRequestMaxMicros *int64
}

type CommercialLimitWindowUsageResult struct {
	BudgetMicros    *int64
	SpentMicros     int64
	RemainingMicros *int64
	Window          QuotaWindow
	Exceeded        bool
}

func (s *APIKeyCommercialLimitService) Check(ctx context.Context, input APIKeyCommercialLimitCheckInput) (APIKeyCommercialLimitCheckResult, error) {
	if input.APIKey == nil || input.APIKey.CommercialLimits == nil || !input.APIKey.CommercialLimits.Enabled {
		return APIKeyCommercialLimitCheckResult{Allowed: true}, nil
	}

	usage, err := s.Usage(ctx, input.APIKey, input.Currency, input.Now)
	if err != nil {
		return APIKeyCommercialLimitCheckResult{}, err
	}

	chargeMicros := input.EstimatedChargeMicros
	if chargeMicros < 0 {
		chargeMicros = 0
	}

	if usage.SingleRequestMaxMicros != nil && chargeMicros > *usage.SingleRequestMaxMicros {
		return APIKeyCommercialLimitCheckResult{
			Allowed: false,
			Message: fmt.Sprintf("api key single request limit exceeded: %d/%d micros", chargeMicros, *usage.SingleRequestMaxMicros),
			Usage:   usage,
		}, ErrAPIKeyCommercialLimitExceeded
	}

	for name, window := range map[string]CommercialLimitWindowUsageResult{
		"total":   usage.Total,
		"daily":   usage.Daily,
		"monthly": usage.Monthly,
	} {
		if window.BudgetMicros == nil {
			continue
		}
		if window.SpentMicros+chargeMicros > *window.BudgetMicros {
			return APIKeyCommercialLimitCheckResult{
				Allowed: false,
				Message: fmt.Sprintf("api key %s budget exceeded: %d/%d micros", name, window.SpentMicros+chargeMicros, *window.BudgetMicros),
				Usage:   usage,
			}, ErrAPIKeyCommercialLimitExceeded
		}
	}

	return APIKeyCommercialLimitCheckResult{Allowed: true, Usage: usage}, nil
}

func (s *APIKeyCommercialLimitService) Usage(ctx context.Context, apiKey *ent.APIKey, currency string, now time.Time) (CommercialLimitUsageResult, error) {
	if apiKey == nil {
		return CommercialLimitUsageResult{}, fmt.Errorf("api key is required")
	}

	limits := normalizeAPIKeyCommercialLimits(apiKey.CommercialLimits, currency)
	if now.IsZero() {
		now = xtime.UTCNow()
	}
	loc := time.UTC
	if s.system != nil {
		loc = s.system.TimeLocation(ctx)
	}

	totalWindow, err := quotaWindow(now, objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}, loc)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}
	dailyWindow, err := quotaWindow(now, objects.APIKeyQuotaPeriod{
		Type: objects.APIKeyQuotaPeriodTypeCalendarDuration,
		CalendarDuration: &objects.APIKeyQuotaCalendarDuration{
			Unit: objects.APIKeyQuotaCalendarDurationUnitDay,
		},
	}, loc)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}
	monthlyWindow, err := quotaWindow(now, objects.APIKeyQuotaPeriod{
		Type: objects.APIKeyQuotaPeriodTypeCalendarDuration,
		CalendarDuration: &objects.APIKeyQuotaCalendarDuration{
			Unit: objects.APIKeyQuotaCalendarDurationUnitMonth,
		},
	}, loc)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}

	totalSpent, err := s.chargedSpendMicros(ctx, apiKey.ID, limits.Currency, totalWindow)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}
	dailySpent, err := s.chargedSpendMicros(ctx, apiKey.ID, limits.Currency, dailyWindow)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}
	monthlySpent, err := s.chargedSpendMicros(ctx, apiKey.ID, limits.Currency, monthlyWindow)
	if err != nil {
		return CommercialLimitUsageResult{}, err
	}

	return CommercialLimitUsageResult{
		APIKeyID: objects.GUID{
			Type: ent.TypeAPIKey,
			ID:   apiKey.ID,
		},
		Currency:               limits.Currency,
		Enabled:                limits.Enabled,
		Total:                  commercialWindowUsage(limits.TotalBudgetMicros, totalSpent, totalWindow),
		Daily:                  commercialWindowUsage(limits.DailyBudgetMicros, dailySpent, dailyWindow),
		Monthly:                commercialWindowUsage(limits.MonthlyBudgetMicros, monthlySpent, monthlyWindow),
		SingleRequestMaxMicros: limits.SingleRequestMaxMicros,
	}, nil
}

func ValidateAPIKeyCommercialLimits(limits *objects.APIKeyCommercialLimits) error {
	if limits == nil {
		return nil
	}

	for name, v := range map[string]*int64{
		"totalBudgetMicros":      limits.TotalBudgetMicros,
		"dailyBudgetMicros":      limits.DailyBudgetMicros,
		"monthlyBudgetMicros":    limits.MonthlyBudgetMicros,
		"singleRequestMaxMicros": limits.SingleRequestMaxMicros,
	} {
		if v != nil && *v < 0 {
			return fmt.Errorf("%s must not be negative", name)
		}
	}

	if limits.Enabled && strings.TrimSpace(limits.Currency) == "" {
		limits.Currency = defaultBillingCurrency
	}

	return nil
}

func normalizeAPIKeyCommercialLimits(limits *objects.APIKeyCommercialLimits, fallbackCurrency string) objects.APIKeyCommercialLimits {
	if limits == nil {
		limits = &objects.APIKeyCommercialLimits{}
	}
	cp := *limits
	cp.Currency = strings.TrimSpace(cp.Currency)
	if cp.Currency == "" {
		cp.Currency = strings.TrimSpace(fallbackCurrency)
	}
	if cp.Currency == "" {
		cp.Currency = defaultBillingCurrency
	}
	return cp
}

func commercialWindowUsage(budget *int64, spent int64, window QuotaWindow) CommercialLimitWindowUsageResult {
	var remaining *int64
	exceeded := false
	if budget != nil {
		value := *budget - spent
		if value < 0 {
			value = 0
		}
		remaining = &value
		exceeded = spent >= *budget
	}

	return CommercialLimitWindowUsageResult{
		BudgetMicros:    budget,
		SpentMicros:     spent,
		RemainingMicros: remaining,
		Window:          window,
		Exceeded:        exceeded,
	}
}

func (s *APIKeyCommercialLimitService) chargedSpendMicros(ctx context.Context, apiKeyID int, currency string, window QuotaWindow) (int64, error) {
	return authz.RunWithSystemBypass(ctx, "api-key-commercial-limit-spend", func(bypassCtx context.Context) (int64, error) {
		q := s.ent.UsageBillingRecord.Query().
			Where(
				usagebillingrecord.APIKeyIDEQ(apiKeyID),
				usagebillingrecord.CurrencyEQ(currency),
				usagebillingrecord.StatusEQ(usagebillingrecord.StatusCharged),
			)

		if window.Start != nil {
			q = q.Where(usagebillingrecord.CreatedAtGTE(*window.Start))
		}

		if window.End != nil {
			if window.EndInclusive {
				q = q.Where(usagebillingrecord.CreatedAtLTE(*window.End))
			} else {
				q = q.Where(usagebillingrecord.CreatedAtLT(*window.End))
			}
		}

		type row struct {
			Total int64 `json:"total"`
		}
		var rows []row
		err := q.Modify(func(s *sql.Selector) {
			s.Select(sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", s.C(usagebillingrecord.FieldChargeAmountMicros)), "total"))
		}).Scan(bypassCtx, &rows)
		if err != nil {
			return 0, err
		}
		if len(rows) == 0 {
			return 0, nil
		}
		return rows[0].Total, nil
	})
}
