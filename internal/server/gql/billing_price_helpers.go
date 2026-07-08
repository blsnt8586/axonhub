package gql

import (
	"context"
	"fmt"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/server/biz"
)

func (r *mutationResolver) saveBillingPriceRule(ctx context.Context, input SaveBillingPriceRuleForm) (*ent.BillingPriceRule, error) {
	if r.pricingService == nil {
		return nil, fmt.Errorf("pricing service is not configured")
	}
	if input.Price == nil {
		return nil, fmt.Errorf("price is required")
	}

	ruleID := 0
	if input.ID != nil {
		if input.ID.Type != ent.TypeBillingPriceRule {
			return nil, fmt.Errorf("id must be a BillingPriceRule ID")
		}
		ruleID = input.ID.ID
	}

	priority := 0
	if input.Priority != nil {
		priority = *input.Priority
	}

	return r.pricingService.SaveBillingPriceRule(ctx, biz.SaveBillingPriceRuleInput{
		ID:           ruleID,
		ScopeType:    input.ScopeType,
		ScopeID:      input.ScopeID,
		ModelPattern: input.ModelPattern,
		Price:        *input.Price,
		Currency:     stringValue(input.Currency),
		Priority:     priority,
		Enabled:      input.Enabled,
		ReferenceID:  stringValue(input.ReferenceID),
	})
}
