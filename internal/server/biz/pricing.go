package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
)

type PricingServiceParams struct {
	fx.In

	Ent *ent.Client
}

type PricingService struct {
	*AbstractService
}

func NewPricingService(params PricingServiceParams) *PricingService {
	return &PricingService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

func (s *PricingService) FindSellPrice(ctx context.Context, projectID int, modelID string) (*ent.BillingPriceRule, error) {
	client := s.entFromContext(ctx)

	rules, err := client.BillingPriceRule.Query().
		Where(billingpricerule.EnabledEQ(true)).
		Order(
			billingpricerule.ByPriority(),
			billingpricerule.ByID(),
		).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query billing price rules: %w", err)
	}

	candidates := []struct {
		scopeType billingpricerule.ScopeType
		scopeID   int
		pattern   string
	}{
		{scopeType: billingpricerule.ScopeTypeProject, scopeID: projectID, pattern: modelID},
		{scopeType: billingpricerule.ScopeTypeProject, scopeID: projectID, pattern: "*"},
		{scopeType: billingpricerule.ScopeTypeGlobal, scopeID: 0, pattern: modelID},
		{scopeType: billingpricerule.ScopeTypeGlobal, scopeID: 0, pattern: "*"},
	}

	for _, candidate := range candidates {
		var best *ent.BillingPriceRule
		for _, rule := range rules {
			if rule.ScopeType != candidate.scopeType || rule.ScopeID != candidate.scopeID || rule.ModelPattern != candidate.pattern {
				continue
			}
			if best == nil || rule.Priority > best.Priority || (rule.Priority == best.Priority && rule.ID > best.ID) {
				best = rule
			}
		}
		if best != nil {
			return best, nil
		}
	}

	return nil, fmt.Errorf("%w: project=%d model=%s", ErrBillingPriceNotFound, projectID, modelID)
}
