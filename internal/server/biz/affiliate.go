package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/affiliateinvitation"
	"github.com/looplj/axonhub/internal/ent/affiliateprofile"
	"github.com/looplj/axonhub/internal/ent/affiliaterebate"
	"github.com/looplj/axonhub/internal/ent/affiliatesetting"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
)

const (
	defaultAffiliateSettingKey = "default"
	affiliateInviteCodePrefix  = "AFF"
)

type AffiliateServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
}

type AffiliateService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
}

func NewAffiliateService(params AffiliateServiceParams) *AffiliateService {
	return &AffiliateService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
	}
}

type SaveAffiliateSettingInput struct {
	Enabled              bool
	DefaultRebateRateBps int
	FreezeDays           int
	MinTransferAmount    decimal.Decimal
	Currency             string
}

type SaveAffiliateProfileInput struct {
	UserID                int
	Status                affiliateprofile.Status
	RebateRateOverrideBps *int
	Notes                 string
}

type BindAffiliateInviteInput struct {
	InviteeUserID int
	InviteCode    string
	Notes         string
}

type CreateAffiliateRebateInput struct {
	SourceType         affiliaterebate.SourceType
	SourceID           int
	PaymentOrderID     int
	UserSubscriptionID int
	InviteeUserID      int
	BaseAmountMicros   int64
	Currency           string
	OccurredAt         time.Time
	IdempotencyKey     string
}

type TransferAffiliateRebatesInput struct {
	UserID int
	Now    time.Time
}

type AffiliateTransferResult struct {
	TransferredCount     int
	TransferredMicros    int64
	Currency             string
	LedgerTransactionIDs []int
}

type AffiliateSummary struct {
	Profile           *ent.AffiliateProfile
	Invitation        *ent.AffiliateInvitation
	InviteeCount      int
	FrozenMicros      int64
	AvailableMicros   int64
	TransferredMicros int64
	Currency          string
	Setting           *ent.AffiliateSetting
}

func (s *AffiliateService) GetOrCreateSetting(ctx context.Context) (*ent.AffiliateSetting, error) {
	setting, err := s.entFromContext(ctx).AffiliateSetting.Query().
		Where(affiliatesetting.KeyEQ(defaultAffiliateSettingKey)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return s.entFromContext(ctx).AffiliateSetting.Create().
			SetKey(defaultAffiliateSettingKey).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load affiliate settings: %w", err)
	}
	return setting, nil
}

func (s *AffiliateService) SaveSetting(ctx context.Context, input SaveAffiliateSettingInput) (*ent.AffiliateSetting, error) {
	if input.DefaultRebateRateBps < 0 || input.DefaultRebateRateBps > 10000 {
		return nil, fmt.Errorf("affiliate rebate rate bps must be between 0 and 10000")
	}
	if input.FreezeDays < 0 {
		return nil, fmt.Errorf("affiliate freeze days must not be negative")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	minMicros, err := decimalToMicros(input.MinTransferAmount)
	if err != nil {
		return nil, err
	}
	if minMicros < 0 {
		return nil, fmt.Errorf("affiliate min transfer amount must not be negative")
	}

	existing, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return nil, err
	}
	return s.entFromContext(ctx).AffiliateSetting.UpdateOneID(existing.ID).
		SetEnabled(input.Enabled).
		SetDefaultRebateRateBps(input.DefaultRebateRateBps).
		SetFreezeDays(input.FreezeDays).
		SetMinTransferMicros(minMicros).
		SetCurrency(input.Currency).
		Save(ctx)
}

func (s *AffiliateService) GetOrCreateProfile(ctx context.Context, userID int) (*ent.AffiliateProfile, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	profile, err := s.entFromContext(ctx).AffiliateProfile.Query().
		Where(affiliateprofile.UserIDEQ(userID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		code, err := newAffiliateInviteCode()
		if err != nil {
			return nil, err
		}
		return s.entFromContext(ctx).AffiliateProfile.Create().
			SetUserID(userID).
			SetInviteCode(code).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load affiliate profile: %w", err)
	}
	return profile, nil
}

func (s *AffiliateService) SaveProfile(ctx context.Context, input SaveAffiliateProfileInput) (*ent.AffiliateProfile, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	if input.Status == "" {
		input.Status = affiliateprofile.StatusActive
	}
	if input.Status != affiliateprofile.StatusActive && input.Status != affiliateprofile.StatusDisabled {
		return nil, fmt.Errorf("unsupported affiliate profile status %q", input.Status)
	}
	if input.RebateRateOverrideBps != nil && (*input.RebateRateOverrideBps < 0 || *input.RebateRateOverrideBps > 10000) {
		return nil, fmt.Errorf("affiliate rebate rate bps must be between 0 and 10000")
	}
	profile, err := s.GetOrCreateProfile(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	update := s.entFromContext(ctx).AffiliateProfile.UpdateOneID(profile.ID).
		SetStatus(input.Status).
		SetNotes(strings.TrimSpace(input.Notes))
	if input.RebateRateOverrideBps != nil {
		update.SetRebateRateOverrideBps(*input.RebateRateOverrideBps)
	} else {
		update.ClearRebateRateOverrideBps()
	}
	return update.Save(ctx)
}

func (s *AffiliateService) BindInvite(ctx context.Context, input BindAffiliateInviteInput) (*ent.AffiliateInvitation, error) {
	if input.InviteeUserID <= 0 {
		return nil, fmt.Errorf("invitee user id is required")
	}
	code := normalizeAffiliateInviteCode(input.InviteCode)
	if code == "" {
		return nil, ErrAffiliateInviteInvalid
	}

	var invitation *ent.AffiliateInvitation
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		profile, err := client.AffiliateProfile.Query().
			Where(affiliateprofile.InviteCodeEQ(code), affiliateprofile.StatusEQ(affiliateprofile.StatusActive)).
			Only(ctx)
		if ent.IsNotFound(err) {
			return ErrAffiliateInviteInvalid
		}
		if err != nil {
			return fmt.Errorf("failed to load inviter profile: %w", err)
		}
		if profile.UserID == input.InviteeUserID {
			return ErrAffiliateSelfInvite
		}
		existing, err := client.AffiliateInvitation.Query().
			Where(affiliateinvitation.InviteeUserIDEQ(input.InviteeUserID)).
			Only(ctx)
		if err == nil {
			invitation = existing
			return ErrAffiliateAlreadyBound
		}
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to check existing affiliate invitation: %w", err)
		}
		if err := s.ensureNoInviteCycle(ctx, profile.UserID, input.InviteeUserID); err != nil {
			return err
		}

		invitation, err = client.AffiliateInvitation.Create().
			SetInviterUserID(profile.UserID).
			SetInviteeUserID(input.InviteeUserID).
			SetInviteCode(profile.InviteCode).
			SetNotes(strings.TrimSpace(input.Notes)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to bind affiliate invitation: %w", err)
		}
		_, err = s.GetOrCreateProfile(ctx, input.InviteeUserID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return invitation, nil
}

func (s *AffiliateService) ensureNoInviteCycle(ctx context.Context, inviterUserID int, inviteeUserID int) error {
	seen := map[int]bool{}
	current := inviterUserID
	for current > 0 {
		if current == inviteeUserID {
			return ErrAffiliateCircularInvite
		}
		if seen[current] {
			return ErrAffiliateCircularInvite
		}
		seen[current] = true
		parent, err := s.entFromContext(ctx).AffiliateInvitation.Query().
			Where(affiliateinvitation.InviteeUserIDEQ(current), affiliateinvitation.StatusEQ(affiliateinvitation.StatusActive)).
			Only(ctx)
		if ent.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to inspect affiliate invitation chain: %w", err)
		}
		current = parent.InviterUserID
	}
	return nil
}

func (s *AffiliateService) CreateRebate(ctx context.Context, input CreateAffiliateRebateInput) (*ent.AffiliateRebate, error) {
	if input.SourceID <= 0 {
		return nil, fmt.Errorf("affiliate rebate source id is required")
	}
	if input.InviteeUserID <= 0 {
		return nil, fmt.Errorf("affiliate rebate invitee user id is required")
	}
	if input.BaseAmountMicros <= 0 {
		return nil, nil
	}
	if input.SourceType == "" {
		return nil, fmt.Errorf("affiliate rebate source type is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = fmt.Sprintf("affiliate_rebate:%s:%d", input.SourceType, input.SourceID)
	}

	setting, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return nil, err
	}
	if !setting.Enabled {
		return nil, nil
	}
	if setting.Currency != input.Currency {
		return nil, nil
	}

	invitation, err := s.entFromContext(ctx).AffiliateInvitation.Query().
		Where(
			affiliateinvitation.InviteeUserIDEQ(input.InviteeUserID),
			affiliateinvitation.StatusEQ(affiliateinvitation.StatusActive),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load affiliate invitation: %w", err)
	}

	profile, err := s.GetOrCreateProfile(ctx, invitation.InviterUserID)
	if err != nil {
		return nil, err
	}
	if profile.Status != affiliateprofile.StatusActive {
		return nil, nil
	}
	rateBps := setting.DefaultRebateRateBps
	if profile.RebateRateOverrideBps != nil {
		rateBps = *profile.RebateRateOverrideBps
	}
	if rateBps <= 0 {
		return nil, nil
	}
	amountMicros := input.BaseAmountMicros * int64(rateBps) / 10000
	if amountMicros <= 0 {
		return nil, nil
	}

	create := s.entFromContext(ctx).AffiliateRebate.Create().
		SetInvitationID(invitation.ID).
		SetInviterUserID(invitation.InviterUserID).
		SetInviteeUserID(invitation.InviteeUserID).
		SetSourceType(input.SourceType).
		SetSourceID(input.SourceID).
		SetBaseAmountMicros(input.BaseAmountMicros).
		SetAmountMicros(amountMicros).
		SetRateBps(rateBps).
		SetCurrency(input.Currency).
		SetStatus(affiliaterebate.StatusFrozen).
		SetFreezeUntil(input.OccurredAt.AddDate(0, 0, setting.FreezeDays)).
		SetIdempotencyKey(input.IdempotencyKey)
	if input.PaymentOrderID > 0 {
		create.SetPaymentOrderID(input.PaymentOrderID)
	}
	if input.UserSubscriptionID > 0 {
		create.SetUserSubscriptionID(input.UserSubscriptionID)
	}
	rebate, err := create.Save(ctx)
	if ent.IsConstraintError(err) {
		return s.entFromContext(ctx).AffiliateRebate.Query().
			Where(affiliaterebate.IdempotencyKeyEQ(input.IdempotencyKey)).
			Only(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create affiliate rebate: %w", err)
	}
	return rebate, nil
}

func (s *AffiliateService) CreateRebateForPaymentOrder(ctx context.Context, order *ent.PaymentOrder) (*ent.AffiliateRebate, error) {
	if order == nil || order.BillingAccountID <= 0 || order.Status != paymentorder.StatusPaid {
		return nil, nil
	}
	account, err := s.entFromContext(ctx).BillingAccount.Get(ctx, order.BillingAccountID)
	if err != nil {
		return nil, err
	}
	if account.OwnerType != billingaccount.OwnerTypeUser {
		return nil, nil
	}
	occurredAt := time.Now().UTC()
	if order.PaidAt != nil {
		occurredAt = *order.PaidAt
	}
	return s.CreateRebate(ctx, CreateAffiliateRebateInput{
		SourceType:       affiliaterebate.SourceTypePaymentOrder,
		SourceID:         order.ID,
		PaymentOrderID:   order.ID,
		InviteeUserID:    account.OwnerID,
		BaseAmountMicros: paymentOrderPayableAmountMicros(order),
		Currency:         order.Currency,
		OccurredAt:       occurredAt,
		IdempotencyKey:   fmt.Sprintf("affiliate_rebate:payment_order:%d", order.ID),
	})
}

func (s *AffiliateService) CreateRebateForSubscription(ctx context.Context, sub *ent.UserSubscription) (*ent.AffiliateRebate, error) {
	if sub == nil || sub.UserID <= 0 || sub.PayableAmountMicros <= 0 {
		return nil, nil
	}
	return s.CreateRebate(ctx, CreateAffiliateRebateInput{
		SourceType:         affiliaterebate.SourceTypeUserSubscription,
		SourceID:           sub.ID,
		UserSubscriptionID: sub.ID,
		InviteeUserID:      sub.UserID,
		BaseAmountMicros:   sub.PayableAmountMicros,
		Currency:           sub.Currency,
		OccurredAt:         sub.CreatedAt,
		IdempotencyKey:     fmt.Sprintf("affiliate_rebate:user_subscription:%d", sub.ID),
	})
}

func (s *AffiliateService) TransferAvailable(ctx context.Context, input TransferAffiliateRebatesInput) (AffiliateTransferResult, error) {
	if input.UserID <= 0 {
		return AffiliateTransferResult{}, fmt.Errorf("user id is required")
	}
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	setting, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return AffiliateTransferResult{}, err
	}

	var result AffiliateTransferResult
	result.Currency = setting.Currency
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		rebates, err := client.AffiliateRebate.Query().
			Where(
				affiliaterebate.InviterUserIDEQ(input.UserID),
				affiliaterebate.CurrencyEQ(setting.Currency),
				affiliaterebate.Or(
					affiliaterebate.StatusEQ(affiliaterebate.StatusAvailable),
					affiliaterebate.And(
						affiliaterebate.StatusEQ(affiliaterebate.StatusFrozen),
						affiliaterebate.FreezeUntilLTE(input.Now),
					),
				),
			).
			All(ctx)
		if err != nil {
			return fmt.Errorf("failed to load transferable affiliate rebates: %w", err)
		}
		if len(rebates) == 0 {
			frozen, err := client.AffiliateRebate.Query().
				Where(
					affiliaterebate.InviterUserIDEQ(input.UserID),
					affiliaterebate.CurrencyEQ(setting.Currency),
					affiliaterebate.StatusEQ(affiliaterebate.StatusFrozen),
					affiliaterebate.FreezeUntilGT(input.Now),
				).
				Count(ctx)
			if err != nil {
				return fmt.Errorf("failed to count frozen affiliate rebates: %w", err)
			}
			if frozen > 0 {
				return ErrAffiliateRebateFrozen
			}
			return nil
		}
		total := int64(0)
		for _, rebate := range rebates {
			total += rebate.AmountMicros
		}
		if total < setting.MinTransferMicros {
			return ErrAffiliateRebateFrozen
		}
		account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(input.UserID))
		if err != nil {
			return err
		}
		for _, rebate := range rebates {
			ledgerTx, err := s.ledgerService.Post(ctx, LedgerPostInput{
				BillingAccountID: account.ID,
				Direction:        ledgertransaction.DirectionCredit,
				Amount:           microsToDecimal(rebate.AmountMicros),
				Currency:         rebate.Currency,
				Type:             ledgertransaction.TypeAffiliateRebate,
				IdempotencyKey:   affiliateRebateLedgerIdempotencyKey(rebate.ID),
				ReferenceType:    "affiliate_rebate",
				ReferenceID:      fmt.Sprint(rebate.ID),
				Memo:             fmt.Sprintf("affiliate rebate from user %d", rebate.InviteeUserID),
				CreatedByType:    ledgertransaction.CreatedByTypeSystem,
				CreatedByID:      fmt.Sprint(input.UserID),
			})
			if err != nil {
				return err
			}
			updated, err := client.AffiliateRebate.Update().
				Where(
					affiliaterebate.IDEQ(rebate.ID),
					affiliaterebate.StatusIn(affiliaterebate.StatusFrozen, affiliaterebate.StatusAvailable),
				).
				SetStatus(affiliaterebate.StatusTransferred).
				SetTransferredAt(input.Now).
				SetLedgerTransactionID(ledgerTx.ID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to mark affiliate rebate transferred: %w", err)
			}
			if updated > 0 {
				result.TransferredCount++
				result.TransferredMicros += rebate.AmountMicros
				result.LedgerTransactionIDs = append(result.LedgerTransactionIDs, ledgerTx.ID)
			}
		}
		return nil
	})
	if err != nil {
		return AffiliateTransferResult{}, err
	}
	return result, nil
}

func (s *AffiliateService) ThawDueRebates(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}
	rebates, err := s.entFromContext(ctx).AffiliateRebate.Query().
		Where(
			affiliaterebate.StatusEQ(affiliaterebate.StatusFrozen),
			affiliaterebate.FreezeUntilLTE(now),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to load thawable affiliate rebates: %w", err)
	}

	thawed := 0
	for _, rebate := range rebates {
		updated, err := s.entFromContext(ctx).AffiliateRebate.Update().
			Where(
				affiliaterebate.IDEQ(rebate.ID),
				affiliaterebate.StatusEQ(affiliaterebate.StatusFrozen),
			).
			SetStatus(affiliaterebate.StatusAvailable).
			Save(ctx)
		if err != nil {
			return thawed, fmt.Errorf("failed to thaw affiliate rebate %d: %w", rebate.ID, err)
		}
		thawed += updated
	}
	return thawed, nil
}

func (s *AffiliateService) Summary(ctx context.Context, userID int, now time.Time) (AffiliateSummary, error) {
	if userID <= 0 {
		return AffiliateSummary{}, fmt.Errorf("user id is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	profile, err := s.GetOrCreateProfile(ctx, userID)
	if err != nil {
		return AffiliateSummary{}, err
	}
	setting, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return AffiliateSummary{}, err
	}
	invitation, err := s.entFromContext(ctx).AffiliateInvitation.Query().
		Where(affiliateinvitation.InviteeUserIDEQ(userID)).
		Only(ctx)
	if ent.IsNotFound(err) {
		invitation = nil
	} else if err != nil {
		return AffiliateSummary{}, err
	}
	inviteeCount, err := s.entFromContext(ctx).AffiliateInvitation.Query().
		Where(affiliateinvitation.InviterUserIDEQ(userID), affiliateinvitation.StatusEQ(affiliateinvitation.StatusActive)).
		Count(ctx)
	if err != nil {
		return AffiliateSummary{}, err
	}
	rebates, err := s.entFromContext(ctx).AffiliateRebate.Query().
		Where(affiliaterebate.InviterUserIDEQ(userID)).
		All(ctx)
	if err != nil {
		return AffiliateSummary{}, err
	}
	summary := AffiliateSummary{Profile: profile, Invitation: invitation, InviteeCount: inviteeCount, Currency: setting.Currency, Setting: setting}
	for _, rebate := range rebates {
		switch rebate.Status {
		case affiliaterebate.StatusTransferred:
			summary.TransferredMicros += rebate.AmountMicros
		case affiliaterebate.StatusFrozen:
			if !rebate.FreezeUntil.After(now) {
				summary.AvailableMicros += rebate.AmountMicros
			} else {
				summary.FrozenMicros += rebate.AmountMicros
			}
		case affiliaterebate.StatusAvailable:
			summary.AvailableMicros += rebate.AmountMicros
		}
	}
	return summary, nil
}

func affiliateRebateLedgerIdempotencyKey(rebateID int) string {
	return fmt.Sprintf("affiliate_rebate_transfer:%d", rebateID)
}

func normalizeAffiliateInviteCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}

func newAffiliateInviteCode() (string, error) {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", fmt.Errorf("failed to generate affiliate invite code: %w", err)
	}
	return affiliateInviteCodePrefix + strings.ToUpper(hex.EncodeToString(buf[:])), nil
}
