package biz

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent/billingaccount"
)

type AdmissionMode string

const (
	AdmissionModeDisabled AdmissionMode = "disabled"
	AdmissionModeWarn     AdmissionMode = "warn"
	AdmissionModeEnforce  AdmissionMode = "enforce"
)

type BillingConfig struct {
	Mode                 AdmissionMode   `conf:"mode" yaml:"mode" json:"mode"`
	Subject              string          `conf:"subject" yaml:"subject" json:"subject"`
	Currency             string          `conf:"currency" yaml:"currency" json:"currency"`
	MinBalance           decimal.Decimal `conf:"min_balance" yaml:"min_balance" json:"min_balance"`
	AllowNegative        bool            `conf:"allow_negative" yaml:"allow_negative" json:"allow_negative"`
	CreditLimitDefault   decimal.Decimal `conf:"credit_limit_default" yaml:"credit_limit_default" json:"credit_limit_default"`
	BlockWhenNoPriceRule bool            `conf:"block_when_no_price_rule" yaml:"block_when_no_price_rule" json:"block_when_no_price_rule"`
}

func (c BillingConfig) normalized() BillingConfig {
	if c.Mode == "" {
		c.Mode = AdmissionModeDisabled
	}
	if c.Subject == "" {
		c.Subject = BillingSubjectTypeProject
	}
	if c.Currency == "" {
		c.Currency = defaultBillingCurrency
	}

	return c
}

type AdmissionServiceParams struct {
	fx.In

	Config                BillingConfig
	BillingAccountService *BillingAccountService
}

type AdmissionService struct {
	config                BillingConfig
	billingAccountService *BillingAccountService
}

func NewAdmissionService(params AdmissionServiceParams) *AdmissionService {
	return &AdmissionService{
		config:                params.Config.normalized(),
		billingAccountService: params.BillingAccountService,
	}
}

type AdmissionCheckInput struct {
	Subject BillingSubject
	ModelID string
}

type AdmissionDecision struct {
	Allowed bool
	Mode    AdmissionMode
	Reason  string
}

func (s *AdmissionService) Check(ctx context.Context, input AdmissionCheckInput) (AdmissionDecision, error) {
	cfg := s.config.normalized()
	decision := AdmissionDecision{Allowed: true, Mode: cfg.Mode}
	if cfg.Mode == AdmissionModeDisabled {
		decision.Reason = "billing disabled"
		return decision, nil
	}
	if input.Subject.Type == "" {
		input.Subject.Type = cfg.Subject
	}

	reason, err := s.checkBillingAccount(ctx, cfg, input.Subject)
	if err == nil {
		decision.Reason = "allowed"
		return decision, nil
	}
	if cfg.Mode == AdmissionModeWarn {
		decision.Reason = reason
		return decision, nil
	}
	if cfg.Mode != AdmissionModeEnforce {
		return AdmissionDecision{}, fmt.Errorf("unsupported admission mode %q", cfg.Mode)
	}

	decision.Allowed = false
	decision.Reason = reason
	return decision, err
}

func (s *AdmissionService) checkBillingAccount(ctx context.Context, cfg BillingConfig, subject BillingSubject) (string, error) {
	account, err := s.billingAccountService.GetBySubject(ctx, subject)
	if err != nil {
		if errors.Is(err, ErrBillingAccountNotFound) {
			return "billing account not found", err
		}
		return "failed to load billing account", err
	}

	switch account.Status {
	case billingaccount.StatusFrozen:
		return "billing account frozen", ErrBillingAccountFrozen
	case billingaccount.StatusClosed:
		return "billing account closed", ErrBillingAccountClosed
	}

	minBalanceMicros, err := decimalToMicros(cfg.MinBalance)
	if err != nil {
		return "invalid minimum balance", err
	}
	if cfg.AllowNegative {
		return "allowed", nil
	}
	if account.BalanceMicros+account.CreditLimitMicros <= minBalanceMicros {
		return "insufficient billing balance", ErrInsufficientBalance
	}

	return "allowed", nil
}
