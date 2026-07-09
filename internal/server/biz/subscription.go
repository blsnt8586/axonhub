package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/ent/promousage"
	"github.com/looplj/axonhub/internal/ent/subscriptionplan"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

const defaultSubscriptionPeriodDays = 30

type SubscriptionServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
	PromoCodeService      *PromoCodeService `optional:"true"`
}

type SubscriptionService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
	promoCodeService      *PromoCodeService
}

func NewSubscriptionService(params SubscriptionServiceParams) *SubscriptionService {
	return &SubscriptionService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
		promoCodeService:      params.PromoCodeService,
	}
}

func (s *SubscriptionService) RegisterScheduledTasks(ctx context.Context, sched *scheduler.Scheduler) error {
	return sched.Register(ctx, scheduler.TaskSpec{
		Name:        "subscription-maintenance",
		Description: "Expire due user subscriptions and reset due subscription usage windows",
		FixRate:     time.Minute,
	}, s.runMaintenanceWithSystemContext)
}

func (s *SubscriptionService) runMaintenanceWithSystemContext(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "subscription-maintenance-worker")
	ctx = ent.NewContext(ctx, s.entFromContext(ctx))

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	now := time.Now().UTC()
	expired, err := s.ExpireDue(ctx, now, 100)
	if err != nil {
		log.Error(ctx, "subscription maintenance failed to expire subscriptions", log.Cause(err))
		return
	}
	reset, err := s.ResetDueUsageWindows(ctx, now, 100)
	if err != nil {
		log.Error(ctx, "subscription maintenance failed to reset usage windows", log.Cause(err))
		return
	}
	if expired > 0 || reset > 0 {
		log.Info(ctx, "subscription maintenance completed", log.Int("expired", expired), log.Int("reset", reset))
	}
}

type SaveSubscriptionPlanInput struct {
	ID                  int
	Name                string
	Description         string
	Period              subscriptionplan.Period
	PeriodDays          int
	Price               decimal.Decimal
	Currency            string
	IncludedAmount      decimal.Decimal
	SupportedModelIDs   []string
	SupportedProjectIDs []int
	SupportedGroupIDs   []int
	AllowWalletFallback *bool
	Status              subscriptionplan.Status
	SortOrder           int
	Metadata            objects.JSONRawMessage
}

type PurchaseSubscriptionPlanInput struct {
	UserID int
	PlanID int
	Now    time.Time
	PromoCode string
}

type AdminAssignSubscriptionInput struct {
	UserID    int
	PlanID    int
	StartsAt  *time.Time
	ExpiresAt *time.Time
	Notes     string
	ActorID   string
}

type ExtendUserSubscriptionInput struct {
	SubscriptionID int
	Days           int
	ExpiresAt      *time.Time
	Notes          string
}

type SubscriptionCoverageInput struct {
	UserID       int
	ProjectID    int
	ModelID      string
	AmountMicros int64
	Now          time.Time
}

type SubscriptionCoverageResult struct {
	Covered              bool
	Subscription         *ent.UserSubscription
	WalletFallbackDenied bool
	DenyReason           string
}

type subscriptionPlanSnapshot struct {
	PlanID               int      `json:"planId"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	Period               string   `json:"period"`
	PeriodDays           int      `json:"periodDays"`
	PriceMicros          int64    `json:"priceMicros"`
	Currency             string   `json:"currency"`
	IncludedAmountMicros int64    `json:"includedAmountMicros"`
	SupportedModelIDs    []string `json:"supportedModelIds"`
	SupportedProjectIDs  []int    `json:"supportedProjectIds"`
	SupportedGroupIDs    []int    `json:"supportedGroupIds"`
	AllowWalletFallback  bool     `json:"allowWalletFallback"`
}

func (s *SubscriptionService) SavePlan(ctx context.Context, input SaveSubscriptionPlanInput) (*ent.SubscriptionPlan, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("subscription plan name is required")
	}
	if input.Period == "" {
		input.Period = subscriptionplan.PeriodMonth
	}
	periodDays := normalizeSubscriptionPeriodDays(input.Period, input.PeriodDays)
	priceMicros, err := decimalToMicros(input.Price)
	if err != nil {
		return nil, err
	}
	if priceMicros < 0 {
		return nil, fmt.Errorf("subscription plan price must not be negative")
	}
	includedMicros, err := decimalToMicros(input.IncludedAmount)
	if err != nil {
		return nil, err
	}
	if includedMicros < 0 {
		return nil, fmt.Errorf("subscription included amount must not be negative")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.Status == "" {
		input.Status = subscriptionplan.StatusEnabled
	}
	allowFallback := true
	if input.AllowWalletFallback != nil {
		allowFallback = *input.AllowWalletFallback
	}

	if input.ID > 0 {
		update := s.entFromContext(ctx).SubscriptionPlan.UpdateOneID(input.ID).
			SetName(name).
			SetDescription(strings.TrimSpace(input.Description)).
			SetPeriod(input.Period).
			SetPeriodDays(periodDays).
			SetPriceMicros(priceMicros).
			SetCurrency(input.Currency).
			SetIncludedAmountMicros(includedMicros).
			SetSupportedModelIds(normalizeStringList(input.SupportedModelIDs)).
			SetSupportedProjectIds(normalizeIntList(input.SupportedProjectIDs)).
			SetSupportedGroupIds(normalizeIntList(input.SupportedGroupIDs)).
			SetAllowWalletFallback(allowFallback).
			SetStatus(input.Status).
			SetSortOrder(input.SortOrder)
		if len(input.Metadata) > 0 {
			update.SetMetadata(input.Metadata)
		} else {
			update.ClearMetadata()
		}
		return update.Save(ctx)
	}

	create := s.entFromContext(ctx).SubscriptionPlan.Create().
		SetName(name).
		SetDescription(strings.TrimSpace(input.Description)).
		SetPeriod(input.Period).
		SetPeriodDays(periodDays).
		SetPriceMicros(priceMicros).
		SetCurrency(input.Currency).
		SetIncludedAmountMicros(includedMicros).
		SetSupportedModelIds(normalizeStringList(input.SupportedModelIDs)).
		SetSupportedProjectIds(normalizeIntList(input.SupportedProjectIDs)).
		SetSupportedGroupIds(normalizeIntList(input.SupportedGroupIDs)).
		SetAllowWalletFallback(allowFallback).
		SetStatus(input.Status).
		SetSortOrder(input.SortOrder)
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}
	return create.Save(ctx)
}

func (s *SubscriptionService) PurchasePlan(ctx context.Context, input PurchaseSubscriptionPlanInput) (*ent.UserSubscription, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
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
	if plan.Status != subscriptionplan.StatusEnabled {
		return nil, fmt.Errorf("subscription plan is not enabled")
	}

	account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(input.UserID))
	if err != nil {
		return nil, err
	}

	var created *ent.UserSubscription
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		payableMicros := plan.PriceMicros
		discountMicros := int64(0)
		var promoApplication *PromoApplication
		if strings.TrimSpace(input.PromoCode) != "" {
			if s.promoCodeService == nil {
				return fmt.Errorf("promo code service is not configured")
			}
			application, err := s.promoCodeService.Apply(ctx, PromoApplyInput{
				PromoQuoteInput: PromoQuoteInput{
					Code:                 input.PromoCode,
					Scope:                promousage.ScopeSubscription,
					UserID:               input.UserID,
					OriginalAmountMicros: plan.PriceMicros,
					Currency:             plan.Currency,
					Now:                  input.Now,
				},
				BillingAccountID: account.ID,
				IdempotencyKey:   fmt.Sprintf("promo:subscription:%d:%d:%d", input.UserID, input.PlanID, input.Now.UnixNano()),
				Status:           promousage.StatusReserved,
			})
			if err != nil {
				return err
			}
			promoApplication = application
			payableMicros = application.PayableAmountMicros
			discountMicros = application.DiscountAmountMicros
		}

		var ledgerTxID int
		if payableMicros > 0 {
			ledgerTx, err := s.ledgerService.Post(ctx, LedgerPostInput{
				BillingAccountID: account.ID,
				Direction:        ledgertransaction.DirectionDebit,
				Amount:           microsToDecimal(payableMicros),
				Currency:         plan.Currency,
				Type:             ledgertransaction.TypeSubscriptionDeduct,
				IdempotencyKey:   fmt.Sprintf("subscription_purchase:%d:%d:%d", input.UserID, input.PlanID, input.Now.UnixNano()),
				ReferenceType:    "subscription_plan",
				ReferenceID:      fmt.Sprint(plan.ID),
				Memo:             fmt.Sprintf("purchase subscription plan %s", plan.Name),
				CreatedByType:    ledgertransaction.CreatedByTypeSystem,
				CreatedByID:      fmt.Sprint(input.UserID),
			})
			if err != nil {
				return err
			}
			ledgerTxID = ledgerTx.ID
		}

		entity, err := s.createSubscriptionFromPlan(ctx, plan, createSubscriptionFromPlanInput{
			UserID:                      input.UserID,
			StartsAt:                    input.Now,
			ExpiresAt:                   input.Now.AddDate(0, 0, plan.PeriodDays),
			PurchaseLedgerTransactionID: ledgerTxID,
			OriginalPriceMicros:         plan.PriceMicros,
			DiscountAmountMicros:        discountMicros,
			PayableAmountMicros:         payableMicros,
			PromoCodeID:                 promoCodeIDFromApplication(promoApplication),
		})
		if err != nil {
			return err
		}
		if promoApplication != nil && promoApplication.Usage != nil {
			update := s.entFromContext(ctx).PromoUsage.UpdateOneID(promoApplication.Usage.ID).
				SetUserSubscriptionID(entity.ID).
				SetStatus(promousage.StatusApplied)
			if ledgerTxID > 0 {
				update.SetLedgerTransactionID(ledgerTxID)
			}
			if _, err := update.Save(ctx); err != nil {
				return fmt.Errorf("failed to attach promo usage to subscription: %w", err)
			}
		}
		created = entity
		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *SubscriptionService) AdminAssign(ctx context.Context, input AdminAssignSubscriptionInput) (*ent.UserSubscription, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	if input.PlanID <= 0 {
		return nil, fmt.Errorf("subscription plan id is required")
	}
	now := time.Now().UTC()
	startsAt := now
	if input.StartsAt != nil {
		startsAt = input.StartsAt.UTC()
	}

	plan, err := s.entFromContext(ctx).SubscriptionPlan.Get(ctx, input.PlanID)
	if err != nil {
		return nil, err
	}
	expiresAt := startsAt.AddDate(0, 0, plan.PeriodDays)
	if input.ExpiresAt != nil {
		expiresAt = input.ExpiresAt.UTC()
	}
	if !expiresAt.After(startsAt) {
		return nil, fmt.Errorf("subscription expiration must be after start")
	}

	assignedByID := 0
	if parsed, ok := parsePositiveInt(input.ActorID); ok {
		assignedByID = parsed
	}
	return s.createSubscriptionFromPlan(ctx, plan, createSubscriptionFromPlanInput{
		UserID:       input.UserID,
		StartsAt:     startsAt,
		ExpiresAt:    expiresAt,
		AssignedByID: assignedByID,
		Notes:        input.Notes,
	})
}

func (s *SubscriptionService) Extend(ctx context.Context, input ExtendUserSubscriptionInput) (*ent.UserSubscription, error) {
	if input.SubscriptionID <= 0 {
		return nil, fmt.Errorf("subscription id is required")
	}
	sub, err := s.entFromContext(ctx).UserSubscription.Get(ctx, input.SubscriptionID)
	if err != nil {
		return nil, err
	}
	expiresAt := sub.ExpiresAt
	if input.ExpiresAt != nil {
		expiresAt = input.ExpiresAt.UTC()
	} else {
		if input.Days <= 0 {
			return nil, fmt.Errorf("extension days must be positive")
		}
		expiresAt = expiresAt.AddDate(0, 0, input.Days)
	}
	if !expiresAt.After(sub.StartsAt) {
		return nil, fmt.Errorf("subscription expiration must be after start")
	}

	update := s.entFromContext(ctx).UserSubscription.UpdateOneID(sub.ID).
		SetExpiresAt(expiresAt).
		SetStatus(usersubscription.StatusActive).
		SetNotes(strings.TrimSpace(input.Notes))
	if sub.CurrentPeriodEnd.After(expiresAt) {
		update.SetCurrentPeriodEnd(expiresAt).SetResetAt(expiresAt)
	}
	return update.Save(ctx)
}

func (s *SubscriptionService) Revoke(ctx context.Context, id int, reason string) (*ent.UserSubscription, error) {
	return s.entFromContext(ctx).UserSubscription.UpdateOneID(id).
		SetStatus(usersubscription.StatusRevoked).
		SetRevokeReason(strings.TrimSpace(reason)).
		Save(ctx)
}

func (s *SubscriptionService) Restore(ctx context.Context, id int) (*ent.UserSubscription, error) {
	sub, err := s.entFromContext(ctx).UserSubscription.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !sub.ExpiresAt.After(time.Now().UTC()) {
		return nil, fmt.Errorf("expired subscription cannot be restored")
	}
	return s.entFromContext(ctx).UserSubscription.UpdateOneID(id).
		SetStatus(usersubscription.StatusActive).
		SetRevokeReason("").
		Save(ctx)
}

func (s *SubscriptionService) ResetUsage(ctx context.Context, id int, now time.Time) (*ent.UserSubscription, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sub, err := s.entFromContext(ctx).UserSubscription.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	nextEnd := now.AddDate(0, 0, sub.PeriodDays)
	if nextEnd.After(sub.ExpiresAt) {
		nextEnd = sub.ExpiresAt
	}
	return s.entFromContext(ctx).UserSubscription.UpdateOneID(id).
		SetUsedAmountMicros(0).
		SetCurrentPeriodStart(now).
		SetCurrentPeriodEnd(nextEnd).
		SetResetAt(nextEnd).
		Save(ctx)
}

func (s *SubscriptionService) ExpireDue(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	subs, err := s.entFromContext(ctx).UserSubscription.Query().
		Where(usersubscription.StatusEQ(usersubscription.StatusActive), usersubscription.ExpiresAtLTE(now)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, err
	}
	for _, sub := range subs {
		if _, err := s.entFromContext(ctx).UserSubscription.UpdateOneID(sub.ID).SetStatus(usersubscription.StatusExpired).Save(ctx); err != nil {
			return 0, err
		}
	}
	return len(subs), nil
}

func (s *SubscriptionService) ResetDueUsageWindows(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	subs, err := s.entFromContext(ctx).UserSubscription.Query().
		Where(
			usersubscription.StatusEQ(usersubscription.StatusActive),
			usersubscription.ResetAtLTE(now),
			usersubscription.ExpiresAtGT(now),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, err
	}
	for _, sub := range subs {
		nextEnd := sub.ResetAt.AddDate(0, 0, sub.PeriodDays)
		if nextEnd.After(sub.ExpiresAt) {
			nextEnd = sub.ExpiresAt
		}
		if _, err := s.entFromContext(ctx).UserSubscription.UpdateOneID(sub.ID).
			SetUsedAmountMicros(0).
			SetCurrentPeriodStart(sub.ResetAt).
			SetCurrentPeriodEnd(nextEnd).
			SetResetAt(nextEnd).
			Save(ctx); err != nil {
			return 0, err
		}
	}
	return len(subs), nil
}

func (s *SubscriptionService) CheckCoverage(ctx context.Context, input SubscriptionCoverageInput) (SubscriptionCoverageResult, error) {
	return s.findSubscriptionCoverage(ctx, input, false)
}

func (s *SubscriptionService) CoverUsage(ctx context.Context, input SubscriptionCoverageInput) (SubscriptionCoverageResult, error) {
	return s.findSubscriptionCoverage(ctx, input, true)
}

type createSubscriptionFromPlanInput struct {
	UserID                      int
	StartsAt                    time.Time
	ExpiresAt                   time.Time
	AssignedByID                int
	PurchaseLedgerTransactionID int
	Notes                       string
	OriginalPriceMicros         int64
	DiscountAmountMicros        int64
	PayableAmountMicros         int64
	PromoCodeID                 int
}

func (s *SubscriptionService) createSubscriptionFromPlan(ctx context.Context, plan *ent.SubscriptionPlan, input createSubscriptionFromPlanInput) (*ent.UserSubscription, error) {
	if !input.ExpiresAt.After(input.StartsAt) {
		return nil, fmt.Errorf("subscription expiration must be after start")
	}
	snapshot, err := newSubscriptionPlanSnapshot(plan)
	if err != nil {
		return nil, err
	}

	create := s.entFromContext(ctx).UserSubscription.Create().
		SetUserID(input.UserID).
		SetPlanID(plan.ID).
		SetPlanSnapshot(snapshot).
		SetStatus(usersubscription.StatusActive).
		SetStartsAt(input.StartsAt).
		SetExpiresAt(input.ExpiresAt).
		SetCurrentPeriodStart(input.StartsAt).
		SetCurrentPeriodEnd(input.ExpiresAt).
		SetResetAt(input.ExpiresAt).
		SetPeriodDays(plan.PeriodDays).
		SetIncludedAmountMicros(plan.IncludedAmountMicros).
		SetUsedAmountMicros(0).
		SetCurrency(plan.Currency).
		SetSupportedModelIds(plan.SupportedModelIds).
		SetSupportedProjectIds(plan.SupportedProjectIds).
		SetSupportedGroupIds(plan.SupportedGroupIds).
		SetAllowWalletFallback(plan.AllowWalletFallback).
		SetOriginalPriceMicros(valueOrDefaultInt64(input.OriginalPriceMicros, plan.PriceMicros)).
		SetDiscountAmountMicros(input.DiscountAmountMicros).
		SetPayableAmountMicros(valueOrDefaultInt64(input.PayableAmountMicros, plan.PriceMicros)).
		SetNotes(strings.TrimSpace(input.Notes))
	if input.AssignedByID > 0 {
		create.SetAssignedByID(input.AssignedByID)
	}
	if input.PurchaseLedgerTransactionID > 0 {
		create.SetPurchaseLedgerTransactionID(input.PurchaseLedgerTransactionID)
	}
	if input.PromoCodeID > 0 {
		create.SetPromoCodeID(input.PromoCodeID)
	}
	return create.Save(ctx)
}

func promoCodeIDFromApplication(app *PromoApplication) int {
	if app == nil || app.Code == nil {
		return 0
	}
	return app.Code.ID
}

func (s *SubscriptionService) findSubscriptionCoverage(ctx context.Context, input SubscriptionCoverageInput, mutate bool) (SubscriptionCoverageResult, error) {
	if input.UserID <= 0 {
		return SubscriptionCoverageResult{}, nil
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	if input.AmountMicros < 0 {
		input.AmountMicros = 0
	}

	subs, err := s.entFromContext(ctx).UserSubscription.Query().
		Where(
			usersubscription.UserIDEQ(input.UserID),
			usersubscription.StatusEQ(usersubscription.StatusActive),
			usersubscription.StartsAtLTE(input.Now),
			usersubscription.ExpiresAtGT(input.Now),
		).
		Order(ent.Asc(usersubscription.FieldCurrentPeriodEnd)).
		All(ctx)
	if err != nil {
		return SubscriptionCoverageResult{}, err
	}

	var fallbackDenied *ent.UserSubscription
	for _, sub := range subs {
		if !subscriptionMatches(sub, input.ProjectID, input.ModelID) {
			continue
		}
		if sub.IncludedAmountMicros > 0 && sub.UsedAmountMicros+input.AmountMicros > sub.IncludedAmountMicros {
			if !sub.AllowWalletFallback && fallbackDenied == nil {
				fallbackDenied = sub
			}
			continue
		}
		if !mutate || input.AmountMicros == 0 {
			return SubscriptionCoverageResult{Covered: true, Subscription: sub}, nil
		}
		predicates := []predicate.UserSubscription{
			usersubscription.IDEQ(sub.ID),
			usersubscription.StatusEQ(usersubscription.StatusActive),
			usersubscription.StartsAtLTE(input.Now),
			usersubscription.ExpiresAtGT(input.Now),
		}
		if sub.IncludedAmountMicros > 0 {
			predicates = append(predicates, usersubscription.UsedAmountMicrosLTE(sub.IncludedAmountMicros-input.AmountMicros))
		}

		updated, err := s.entFromContext(ctx).UserSubscription.Update().
			Where(predicates...).
			AddUsedAmountMicros(input.AmountMicros).
			Save(ctx)
		if err != nil {
			return SubscriptionCoverageResult{}, err
		}
		if updated == 0 {
			continue
		}
		updatedSub, err := s.entFromContext(ctx).UserSubscription.Get(ctx, sub.ID)
		if err != nil {
			return SubscriptionCoverageResult{}, err
		}
		return SubscriptionCoverageResult{Covered: true, Subscription: updatedSub}, nil
	}

	if fallbackDenied != nil {
		return SubscriptionCoverageResult{
			Subscription:         fallbackDenied,
			WalletFallbackDenied: true,
			DenyReason:           "subscription quota exhausted and wallet fallback is disabled",
		}, nil
	}

	return SubscriptionCoverageResult{}, nil
}

func subscriptionMatches(sub *ent.UserSubscription, projectID int, modelID string) bool {
	if len(sub.SupportedProjectIds) > 0 && !slices.Contains(sub.SupportedProjectIds, projectID) {
		return false
	}
	if len(sub.SupportedModelIds) > 0 && !slices.Contains(sub.SupportedModelIds, modelID) {
		return false
	}
	return true
}

func normalizeSubscriptionPeriodDays(period subscriptionplan.Period, days int) int {
	if days > 0 {
		return days
	}
	switch period {
	case subscriptionplan.PeriodDay:
		return 1
	case subscriptionplan.PeriodYear:
		return 365
	default:
		return defaultSubscriptionPeriodDays
	}
}

func newSubscriptionPlanSnapshot(plan *ent.SubscriptionPlan) (objects.JSONRawMessage, error) {
	snapshot := subscriptionPlanSnapshot{
		PlanID:               plan.ID,
		Name:                 plan.Name,
		Description:          plan.Description,
		Period:               string(plan.Period),
		PeriodDays:           plan.PeriodDays,
		PriceMicros:          plan.PriceMicros,
		Currency:             plan.Currency,
		IncludedAmountMicros: plan.IncludedAmountMicros,
		SupportedModelIDs:    plan.SupportedModelIds,
		SupportedProjectIDs:  plan.SupportedProjectIds,
		SupportedGroupIDs:    plan.SupportedGroupIds,
		AllowWalletFallback:  plan.AllowWalletFallback,
	}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return objects.JSONRawMessage(raw), nil
}

func normalizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" && !slices.Contains(out, trimmed) {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeIntList(values []int) []int {
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

func valueOrDefaultInt64(value int64, fallback int64) int64 {
	if value == 0 {
		return fallback
	}
	return value
}
