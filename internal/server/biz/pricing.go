package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/objects"
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

type SaveBillingPriceRuleInput struct {
	ID           int
	ScopeType    billingpricerule.ScopeType
	ScopeID      int
	ModelPattern string
	Price        objects.ModelPrice
	Currency     string
	Priority     int
	Enabled      *bool
	ReferenceID  string
}

func (s *PricingService) SaveBillingPriceRule(ctx context.Context, input SaveBillingPriceRuleInput) (*ent.BillingPriceRule, error) {
	if input.ScopeType == "" {
		input.ScopeType = billingpricerule.ScopeTypeGlobal
	}
	input.ModelPattern = strings.TrimSpace(input.ModelPattern)
	if err := validateBillingPriceRuleInput(input); err != nil {
		return nil, err
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.ReferenceID == "" {
		input.ReferenceID = fmt.Sprintf("sell:%s:%d:%s:%d", input.ScopeType, input.ScopeID, input.ModelPattern, time.Now().UTC().UnixNano())
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}

	client := s.entFromContext(ctx)
	if input.ID > 0 {
		updated, err := client.BillingPriceRule.UpdateOneID(input.ID).
			SetScopeType(input.ScopeType).
			SetScopeID(input.ScopeID).
			SetModelPattern(input.ModelPattern).
			SetPrice(input.Price).
			SetCurrency(input.Currency).
			SetPriority(input.Priority).
			SetEnabled(enabled).
			SetReferenceID(input.ReferenceID).
			Save(ctx)
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("billing price rule not found: %d", input.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to update billing price rule: %w", err)
		}

		return updated, nil
	}

	created, err := client.BillingPriceRule.Create().
		SetScopeType(input.ScopeType).
		SetScopeID(input.ScopeID).
		SetModelPattern(input.ModelPattern).
		SetPrice(input.Price).
		SetCurrency(input.Currency).
		SetPriority(input.Priority).
		SetEnabled(enabled).
		SetReferenceID(input.ReferenceID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing price rule: %w", err)
	}

	return created, nil
}

func (s *PricingService) DeleteBillingPriceRule(ctx context.Context, id int) (bool, error) {
	if id <= 0 {
		return false, fmt.Errorf("billing price rule id is required")
	}

	err := s.entFromContext(ctx).BillingPriceRule.DeleteOneID(id).Exec(ctx)
	if ent.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to delete billing price rule: %w", err)
	}

	return true, nil
}

func validateBillingPriceRuleInput(input SaveBillingPriceRuleInput) error {
	switch input.ScopeType {
	case billingpricerule.ScopeTypeGlobal:
		if input.ScopeID != 0 {
			return fmt.Errorf("global billing price rules must use scope_id 0")
		}
	case billingpricerule.ScopeTypeProject:
		if input.ScopeID <= 0 {
			return fmt.Errorf("project billing price rules require a positive scope_id")
		}
	default:
		return fmt.Errorf("unsupported billing price rule scope_type %q", input.ScopeType)
	}

	if input.ModelPattern == "" {
		return fmt.Errorf("model_pattern is required")
	}
	if len(input.Price.Items) == 0 {
		return fmt.Errorf("price.items is required")
	}
	if err := input.Price.Validate(); err != nil {
		return fmt.Errorf("invalid model price: %w", err)
	}

	return nil
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
