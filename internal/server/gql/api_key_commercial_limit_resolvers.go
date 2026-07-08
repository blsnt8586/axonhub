package gql

import "github.com/looplj/axonhub/internal/server/biz"

func toGQLAPIKeyCommercialLimitUsage(usage biz.CommercialLimitUsageResult) *APIKeyCommercialLimitUsage {
	return &APIKeyCommercialLimitUsage{
		APIKeyID:               usage.APIKeyID,
		Currency:               usage.Currency,
		Enabled:                usage.Enabled,
		Total:                  toGQLAPIKeyCommercialLimitWindowUsage(usage.Total),
		Daily:                  toGQLAPIKeyCommercialLimitWindowUsage(usage.Daily),
		Monthly:                toGQLAPIKeyCommercialLimitWindowUsage(usage.Monthly),
		SingleRequestMaxMicros: usage.SingleRequestMaxMicros,
	}
}

func toGQLAPIKeyCommercialLimitWindowUsage(usage biz.CommercialLimitWindowUsageResult) *APIKeyCommercialLimitWindowUsage {
	return &APIKeyCommercialLimitWindowUsage{
		BudgetMicros:    usage.BudgetMicros,
		SpentMicros:     usage.SpentMicros,
		RemainingMicros: usage.RemainingMicros,
		Window: &APIKeyQuotaWindow{
			Start: usage.Window.Start,
			End:   usage.Window.End,
		},
		Exceeded: usage.Exceeded,
	}
}
