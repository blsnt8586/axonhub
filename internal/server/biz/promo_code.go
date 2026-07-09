package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/ent/promocode"
	"github.com/looplj/axonhub/internal/ent/promousage"
	"github.com/looplj/axonhub/internal/objects"
)

type PromoCodeServiceParams struct {
	fx.In

	Ent *ent.Client
}

type PromoCodeService struct {
	*AbstractService
}

func NewPromoCodeService(params PromoCodeServiceParams) *PromoCodeService {
	return &PromoCodeService{AbstractService: &AbstractService{db: params.Ent}}
}

type SavePromoCodeInput struct {
	ID                    int
	Code                  string
	Description           string
	DiscountType          promocode.DiscountType
	DiscountAmount        decimal.Decimal
	DiscountPercentBps    int
	Scope                 promocode.Scope
	Status                promocode.Status
	Currency              string
	MaxUses               int
	PerUserLimit          int
	StartsAt              *time.Time
	ExpiresAt             *time.Time
	Notes                 string
	Metadata              objects.JSONRawMessage
	ActorID               string
}

type UpdatePromoCodeStatusInput struct {
	CodeID int
	Status promocode.Status
	Notes  string
}

type PromoQuoteInput struct {
	Code                 string
	Scope                promousage.Scope
	UserID               int
	OriginalAmountMicros int64
	Currency             string
	Now                  time.Time
}

type QuoteRechargePromoInput struct {
	Code     string
	UserID   int
	Amount   decimal.Decimal
	Currency string
}

type QuoteSubscriptionPromoInput struct {
	Code   string
	UserID int
	PlanID int
	Now    time.Time
}

type PromoApplyInput struct {
	PromoQuoteInput
	BillingAccountID int
	PaymentOrderID   int
	UserSubscriptionID int
	LedgerTransactionID int
	IdempotencyKey   string
	Status           promousage.Status
}

type PromoApplication struct {
	Code                 *ent.PromoCode
	Usage                *ent.PromoUsage
	OriginalAmountMicros int64
	DiscountAmountMicros int64
	PayableAmountMicros  int64
	Currency             string
}

func (s *PromoCodeService) Save(ctx context.Context, input SavePromoCodeInput) (*ent.PromoCode, error) {
	code := normalizePromoCode(input.Code)
	if code == "" {
		return nil, fmt.Errorf("promo code is required")
	}
	if input.DiscountType == "" {
		input.DiscountType = promocode.DiscountTypeAmount
	}
	if input.Scope == "" {
		input.Scope = promocode.ScopeAll
	}
	if input.Status == "" {
		input.Status = promocode.StatusActive
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	amountMicros, err := decimalToMicros(input.DiscountAmount)
	if err != nil {
		return nil, err
	}
	if amountMicros < 0 {
		return nil, fmt.Errorf("promo discount amount must not be negative")
	}
	if input.DiscountPercentBps < 0 || input.DiscountPercentBps > 10000 {
		return nil, fmt.Errorf("promo discount percent bps must be between 0 and 10000")
	}
	if input.DiscountType == promocode.DiscountTypeAmount && amountMicros <= 0 {
		return nil, fmt.Errorf("promo discount amount must be positive")
	}
	if input.DiscountType == promocode.DiscountTypePercent && input.DiscountPercentBps <= 0 {
		return nil, fmt.Errorf("promo discount percent bps must be positive")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC()) {
		return nil, fmt.Errorf("promo code expiration must be in the future")
	}
	if input.StartsAt != nil && input.ExpiresAt != nil && !input.ExpiresAt.After(*input.StartsAt) {
		return nil, fmt.Errorf("promo code expiration must be after start")
	}
	if input.MaxUses < 0 || input.PerUserLimit < 0 {
		return nil, fmt.Errorf("promo usage limits must not be negative")
	}

	if input.ID > 0 {
		update := s.entFromContext(ctx).PromoCode.UpdateOneID(input.ID).
			SetDescription(strings.TrimSpace(input.Description)).
			SetDiscountType(input.DiscountType).
			SetDiscountAmountMicros(amountMicros).
			SetDiscountPercentBps(input.DiscountPercentBps).
			SetScope(input.Scope).
			SetStatus(input.Status).
			SetCurrency(input.Currency).
			SetMaxUses(input.MaxUses).
			SetPerUserLimit(input.PerUserLimit).
			SetNotes(strings.TrimSpace(input.Notes))
		if input.StartsAt != nil {
			update.SetStartsAt(*input.StartsAt)
		} else {
			update.ClearStartsAt()
		}
		if input.ExpiresAt != nil {
			update.SetExpiresAt(*input.ExpiresAt)
		} else {
			update.ClearExpiresAt()
		}
		if len(input.Metadata) > 0 {
			update.SetMetadata(input.Metadata)
		} else {
			update.ClearMetadata()
		}
		return update.Save(ctx)
	}

	create := s.entFromContext(ctx).PromoCode.Create().
		SetCode(code).
		SetDescription(strings.TrimSpace(input.Description)).
		SetDiscountType(input.DiscountType).
		SetDiscountAmountMicros(amountMicros).
		SetDiscountPercentBps(input.DiscountPercentBps).
		SetScope(input.Scope).
		SetStatus(input.Status).
		SetCurrency(input.Currency).
		SetMaxUses(input.MaxUses).
		SetPerUserLimit(input.PerUserLimit).
		SetNotes(strings.TrimSpace(input.Notes))
	if input.StartsAt != nil {
		create.SetStartsAt(*input.StartsAt)
	}
	if input.ExpiresAt != nil {
		create.SetExpiresAt(*input.ExpiresAt)
	}
	if actorID, ok := parsePositiveInt(input.ActorID); ok {
		create.SetCreatedByID(actorID)
	}
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}

	return create.Save(ctx)
}

func (s *PromoCodeService) UpdateStatus(ctx context.Context, input UpdatePromoCodeStatusInput) (*ent.PromoCode, error) {
	if input.CodeID <= 0 {
		return nil, fmt.Errorf("promo code id is required")
	}
	if input.Status != promocode.StatusActive && input.Status != promocode.StatusDisabled && input.Status != promocode.StatusExpired {
		return nil, fmt.Errorf("unsupported promo code status %q", input.Status)
	}
	update := s.entFromContext(ctx).PromoCode.UpdateOneID(input.CodeID).SetStatus(input.Status)
	if strings.TrimSpace(input.Notes) != "" {
		update.SetNotes(strings.TrimSpace(input.Notes))
	}
	code, err := update.Save(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrPromoCodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update promo code status: %w", err)
	}
	return code, nil
}

func (s *PromoCodeService) Delete(ctx context.Context, codeID int) error {
	if codeID <= 0 {
		return fmt.Errorf("promo code id is required")
	}
	used, err := s.entFromContext(ctx).PromoUsage.Query().Where(promousage.PromoCodeIDEQ(codeID)).Exist(ctx)
	if err != nil {
		return fmt.Errorf("failed to check promo usage: %w", err)
	}
	if used {
		_, err = s.entFromContext(ctx).PromoCode.UpdateOneID(codeID).SetStatus(promocode.StatusDisabled).Save(ctx)
		return err
	}
	return s.entFromContext(ctx).PromoCode.DeleteOneID(codeID).Exec(ctx)
}

func (s *PromoCodeService) Quote(ctx context.Context, input PromoQuoteInput) (*PromoApplication, error) {
	if strings.TrimSpace(input.Code) == "" {
		return &PromoApplication{OriginalAmountMicros: input.OriginalAmountMicros, PayableAmountMicros: input.OriginalAmountMicros, Currency: normalizedCurrency(input.Currency)}, nil
	}
	if input.OriginalAmountMicros <= 0 {
		return nil, fmt.Errorf("promo original amount must be positive")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("promo user id is required")
	}
	if input.Scope == "" {
		return nil, fmt.Errorf("promo scope is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}

	code, err := s.entFromContext(ctx).PromoCode.Query().Where(promocode.CodeEQ(normalizePromoCode(input.Code))).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrPromoCodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query promo code: %w", err)
	}
	if err := validatePromoCodeForUse(ctx, s.entFromContext(ctx), code, input); err != nil {
		return nil, err
	}

	discount := calculatePromoDiscountMicros(code, input.OriginalAmountMicros)
	if discount <= 0 {
		return nil, fmt.Errorf("promo code has no effective discount")
	}
	payable := input.OriginalAmountMicros - discount
	if payable < 0 {
		payable = 0
	}

	return &PromoApplication{
		Code:                 code,
		OriginalAmountMicros: input.OriginalAmountMicros,
		DiscountAmountMicros: discount,
		PayableAmountMicros:  payable,
		Currency:             code.Currency,
	}, nil
}

func (s *PromoCodeService) QuoteRecharge(ctx context.Context, input QuoteRechargePromoInput) (*PromoApplication, error) {
	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}
	return s.Quote(ctx, PromoQuoteInput{
		Code:                 input.Code,
		Scope:                promousage.ScopeRecharge,
		UserID:               input.UserID,
		OriginalAmountMicros: amountMicros,
		Currency:             input.Currency,
	})
}

func (s *PromoCodeService) QuoteSubscription(ctx context.Context, input QuoteSubscriptionPromoInput) (*PromoApplication, error) {
	if input.PlanID <= 0 {
		return nil, fmt.Errorf("subscription plan id is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	plan, err := s.entFromContext(ctx).SubscriptionPlan.Get(ctx, input.PlanID)
	if err != nil {
		return nil, err
	}
	return s.Quote(ctx, PromoQuoteInput{
		Code:                 input.Code,
		Scope:                promousage.ScopeSubscription,
		UserID:               input.UserID,
		OriginalAmountMicros: plan.PriceMicros,
		Currency:             plan.Currency,
		Now:                  input.Now,
	})
}

func (s *PromoCodeService) Apply(ctx context.Context, input PromoApplyInput) (*PromoApplication, error) {
	if strings.TrimSpace(input.Code) == "" {
		return &PromoApplication{OriginalAmountMicros: input.OriginalAmountMicros, PayableAmountMicros: input.OriginalAmountMicros, Currency: normalizedCurrency(input.Currency)}, nil
	}
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return nil, fmt.Errorf("promo idempotency key is required")
	}
	if input.Status == "" {
		input.Status = promousage.StatusReserved
	}

	var app *PromoApplication
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		if existing, err := client.PromoUsage.Query().Where(promousage.IdempotencyKeyEQ(input.IdempotencyKey)).Only(ctx); err == nil {
			code, err := client.PromoCode.Get(ctx, existing.PromoCodeID)
			if err != nil {
				return err
			}
			app = &PromoApplication{Code: code, Usage: existing, OriginalAmountMicros: existing.OriginalAmountMicros, DiscountAmountMicros: existing.DiscountAmountMicros, PayableAmountMicros: existing.PayableAmountMicros, Currency: existing.Currency}
			return nil
		} else if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query promo usage: %w", err)
		}

		quote, err := s.Quote(ctx, input.PromoQuoteInput)
		if err != nil {
			return err
		}
		code := quote.Code

		predicates := []predicate.PromoCode{promocode.IDEQ(code.ID), promocode.StatusEQ(promocode.StatusActive)}
		if code.MaxUses > 0 {
			predicates = append(predicates, promocode.UsedCountLT(code.MaxUses))
		}
		updated, err := client.PromoCode.Update().Where(predicates...).AddUsedCount(1).Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to reserve promo code: %w", err)
		}
		if updated == 0 {
			return ErrPromoCodeExhausted
		}

		snapshot, err := newPromoCodeSnapshot(code)
		if err != nil {
			return err
		}
		create := client.PromoUsage.Create().
			SetPromoCodeID(code.ID).
			SetCode(code.Code).
			SetCodeSnapshot(snapshot).
			SetUserID(input.UserID).
			SetScope(input.Scope).
			SetStatus(input.Status).
			SetOriginalAmountMicros(quote.OriginalAmountMicros).
			SetDiscountAmountMicros(quote.DiscountAmountMicros).
			SetPayableAmountMicros(quote.PayableAmountMicros).
			SetCurrency(quote.Currency).
			SetIdempotencyKey(input.IdempotencyKey)
		if input.BillingAccountID > 0 {
			create.SetBillingAccountID(input.BillingAccountID)
		}
		if input.PaymentOrderID > 0 {
			create.SetPaymentOrderID(input.PaymentOrderID)
		}
		if input.UserSubscriptionID > 0 {
			create.SetUserSubscriptionID(input.UserSubscriptionID)
		}
		if input.LedgerTransactionID > 0 {
			create.SetLedgerTransactionID(input.LedgerTransactionID)
		}
		usage, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create promo usage: %w", err)
		}
		app = quote
		app.Usage = usage
		return nil
	})
	if err != nil {
		return nil, err
	}
	return app, nil
}

func validatePromoCodeForUse(ctx context.Context, client *ent.Client, code *ent.PromoCode, input PromoQuoteInput) error {
	switch code.Status {
	case promocode.StatusActive:
	case promocode.StatusDisabled:
		return ErrPromoCodeDisabled
	case promocode.StatusExpired:
		return ErrPromoCodeExpired
	default:
		return fmt.Errorf("unsupported promo code status %q", code.Status)
	}
	if code.StartsAt != nil && code.StartsAt.After(input.Now) {
		return ErrPromoCodeDisabled
	}
	if code.ExpiresAt != nil && !code.ExpiresAt.After(input.Now) {
		return ErrPromoCodeExpired
	}
	if code.Scope != promocode.ScopeAll && string(code.Scope) != string(input.Scope) {
		return ErrPromoCodeScopeMismatch
	}
	if code.Currency != normalizedCurrency(input.Currency) {
		return fmt.Errorf("promo currency %s does not match %s", code.Currency, normalizedCurrency(input.Currency))
	}
	if code.MaxUses > 0 && code.UsedCount >= code.MaxUses {
		return ErrPromoCodeExhausted
	}
	if code.PerUserLimit > 0 {
		used, err := client.PromoUsage.Query().Where(
			promousage.PromoCodeIDEQ(code.ID),
			promousage.UserIDEQ(input.UserID),
		).Count(ctx)
		if err != nil {
			return fmt.Errorf("failed to count promo usages: %w", err)
		}
		if used >= code.PerUserLimit {
			return ErrPromoCodeReused
		}
	}
	return nil
}

func calculatePromoDiscountMicros(code *ent.PromoCode, originalMicros int64) int64 {
	if originalMicros <= 0 {
		return 0
	}
	var discount int64
	switch code.DiscountType {
	case promocode.DiscountTypeAmount:
		discount = code.DiscountAmountMicros
	case promocode.DiscountTypePercent:
		discount = originalMicros * int64(code.DiscountPercentBps) / 10000
	}
	if discount > originalMicros {
		return originalMicros
	}
	if discount < 0 {
		return 0
	}
	return discount
}

func normalizePromoCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func normalizedCurrency(currency string) string {
	if currency == "" {
		return defaultBillingCurrency
	}
	return currency
}

func newPromoCodeSnapshot(code *ent.PromoCode) (objects.JSONRawMessage, error) {
	snapshot := map[string]any{
		"id":                   code.ID,
		"code":                 code.Code,
		"discountType":         code.DiscountType,
		"discountAmountMicros": code.DiscountAmountMicros,
		"discountPercentBps":   code.DiscountPercentBps,
		"scope":                code.Scope,
		"currency":             code.Currency,
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal promo snapshot: %w", err)
	}
	return objects.JSONRawMessage(raw), nil
}
