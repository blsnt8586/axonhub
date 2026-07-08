package biz

import (
	"context"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
)

type AdmissionMode string

const (
	AdmissionModeDisabled AdmissionMode = "disabled"
	AdmissionModeWarn     AdmissionMode = "warn"
	AdmissionModeEnforce  AdmissionMode = "enforce"
)

type AdmissionCode string

const (
	AdmissionCodeAllowed               AdmissionCode = "allowed"
	AdmissionCodeBillingDisabled       AdmissionCode = "billing_disabled"
	AdmissionCodeAccountNotFound       AdmissionCode = "billing_account_not_found"
	AdmissionCodeAccountLoadFailed     AdmissionCode = "billing_account_load_failed"
	AdmissionCodeAccountFrozen         AdmissionCode = "billing_account_frozen"
	AdmissionCodeAccountClosed         AdmissionCode = "billing_account_closed"
	AdmissionCodeInvalidMinimumBalance AdmissionCode = "invalid_minimum_balance"
	AdmissionCodeInsufficientBalance   AdmissionCode = "insufficient_billing_balance"
)

type BillingConfig struct {
	Mode                        AdmissionMode   `conf:"mode" yaml:"mode" json:"mode"`
	Subject                     string          `conf:"subject" yaml:"subject" json:"subject"`
	Currency                    string          `conf:"currency" yaml:"currency" json:"currency"`
	MinBalance                  decimal.Decimal `conf:"min_balance" yaml:"min_balance" json:"min_balance"`
	AllowNegative               bool            `conf:"allow_negative" yaml:"allow_negative" json:"allow_negative"`
	CreditLimitDefault          decimal.Decimal `conf:"credit_limit_default" yaml:"credit_limit_default" json:"credit_limit_default"`
	HoldDefaultAmount           decimal.Decimal `conf:"hold_default_amount" yaml:"hold_default_amount" json:"hold_default_amount"`
	HoldTTLSeconds              int             `conf:"hold_ttl_seconds" yaml:"hold_ttl_seconds" json:"hold_ttl_seconds"`
	BlockWhenNoPriceRule        bool            `conf:"block_when_no_price_rule" yaml:"block_when_no_price_rule" json:"block_when_no_price_rule"`
	OutboxWorkerIntervalSeconds int             `conf:"outbox_worker_interval_seconds" yaml:"outbox_worker_interval_seconds" json:"outbox_worker_interval_seconds"`
	OutboxBatchSize             int             `conf:"outbox_batch_size" yaml:"outbox_batch_size" json:"outbox_batch_size"`
	OutboxMaxAttempts           int             `conf:"outbox_max_attempts" yaml:"outbox_max_attempts" json:"outbox_max_attempts"`
	OutboxRetryDelaySeconds     int             `conf:"outbox_retry_delay_seconds" yaml:"outbox_retry_delay_seconds" json:"outbox_retry_delay_seconds"`
}

func (c BillingConfig) normalized() BillingConfig {
	if c.Mode == "" {
		c.Mode = AdmissionModeDisabled
	}
	if c.Subject == "" {
		c.Subject = BillingSubjectTypeUser
	}
	if c.Currency == "" {
		c.Currency = defaultBillingCurrency
	}
	if c.OutboxWorkerIntervalSeconds <= 0 {
		c.OutboxWorkerIntervalSeconds = 60
	}
	if c.HoldDefaultAmount.IsZero() {
		c.HoldDefaultAmount = decimal.RequireFromString("0.000001")
	}
	if c.HoldTTLSeconds <= 0 {
		c.HoldTTLSeconds = int(defaultHoldTTL.Seconds())
	}
	if c.OutboxBatchSize <= 0 {
		c.OutboxBatchSize = 100
	}
	if c.OutboxMaxAttempts <= 0 {
		c.OutboxMaxAttempts = 10
	}
	if c.OutboxRetryDelaySeconds <= 0 {
		c.OutboxRetryDelaySeconds = 60
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

func BillingSubjectForAPIKey(cfg BillingConfig, apiKey *ent.APIKey, projectID int) (BillingSubject, bool) {
	switch cfg.normalized().Subject {
	case BillingSubjectTypeUser:
		if apiKey != nil && apiKey.UserID > 0 {
			return UserBillingSubject(apiKey.UserID), true
		}
		return BillingSubject{}, false
	case BillingSubjectTypeProject:
		if projectID > 0 {
			return ProjectBillingSubject(projectID), true
		}
		return BillingSubject{}, false
	default:
		return BillingSubject{}, false
	}
}

func (s *AdmissionService) BillingSubjectForAPIKey(apiKey *ent.APIKey, projectID int) (BillingSubject, bool) {
	return BillingSubjectForAPIKey(s.config, apiKey, projectID)
}

type AdmissionCheckInput struct {
	Subject BillingSubject
	ModelID string
}

type AdmissionDecision struct {
	Allowed bool
	Mode    AdmissionMode
	Code    AdmissionCode
	Reason  string
}

func (s *AdmissionService) Check(ctx context.Context, input AdmissionCheckInput) (AdmissionDecision, error) {
	cfg := s.config.normalized()
	decision := AdmissionDecision{Allowed: true, Mode: cfg.Mode, Code: AdmissionCodeAllowed}
	if cfg.Mode == AdmissionModeDisabled {
		decision.Code = AdmissionCodeBillingDisabled
		decision.Reason = "billing disabled"
		return decision, nil
	}
	if input.Subject.Type == "" {
		input.Subject.Type = cfg.Subject
	}

	code, reason, err := s.checkBillingAccount(ctx, cfg, input.Subject)
	if err == nil {
		decision.Code = code
		decision.Reason = "allowed"
		return decision, nil
	}
	if cfg.Mode == AdmissionModeWarn {
		decision.Code = code
		decision.Reason = reason
		return decision, nil
	}
	if cfg.Mode != AdmissionModeEnforce {
		return AdmissionDecision{}, fmt.Errorf("unsupported admission mode %q", cfg.Mode)
	}

	decision.Allowed = false
	decision.Code = code
	decision.Reason = reason
	return decision, err
}

func (s *AdmissionService) checkBillingAccount(ctx context.Context, cfg BillingConfig, subject BillingSubject) (AdmissionCode, string, error) {
	account, err := s.billingAccountService.GetBySubject(ctx, subject)
	if err != nil {
		if errors.Is(err, ErrBillingAccountNotFound) {
			return AdmissionCodeAccountNotFound, "billing account not found", err
		}
		return AdmissionCodeAccountLoadFailed, "failed to load billing account", err
	}

	switch account.Status {
	case billingaccount.StatusFrozen:
		return AdmissionCodeAccountFrozen, "billing account frozen", ErrBillingAccountFrozen
	case billingaccount.StatusClosed:
		return AdmissionCodeAccountClosed, "billing account closed", ErrBillingAccountClosed
	}

	minBalanceMicros, err := decimalToMicros(cfg.MinBalance)
	if err != nil {
		return AdmissionCodeInvalidMinimumBalance, "invalid minimum balance", err
	}
	if cfg.AllowNegative {
		return AdmissionCodeAllowed, "allowed", nil
	}
	if account.BalanceMicros+account.CreditLimitMicros-account.HeldBalanceMicros <= minBalanceMicros {
		return AdmissionCodeInsufficientBalance, "insufficient billing balance", ErrInsufficientBalance
	}

	return AdmissionCodeAllowed, "allowed", nil
}
