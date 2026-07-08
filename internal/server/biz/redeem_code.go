package biz

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/redeemcode"
)

const (
	defaultRedeemCodeCount = 1
	maxRedeemCodeBatchSize = 500
)

type RedeemCodeServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
}

type RedeemCodeService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
}

func NewRedeemCodeService(params RedeemCodeServiceParams) *RedeemCodeService {
	return &RedeemCodeService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
	}
}

type CreateRedeemCodesInput struct {
	Count     int
	Type      redeemcode.Type
	Amount    decimal.Decimal
	Currency  string
	ExpiresAt *time.Time
	Notes     string
	Prefix    string
	ActorID   string
	BatchID   string
}

type RedeemCodeInput struct {
	Code          string
	UserID        int
	CreatedByType ledgertransaction.CreatedByType
	CreatedByID   string
}

type AdminCreateAndRedeemCodeInput struct {
	UserID    int
	Amount    decimal.Decimal
	Currency  string
	ExpiresAt *time.Time
	Notes     string
	ActorID   string
}

type UpdateRedeemCodeStatusInput struct {
	CodeID int
	Status redeemcode.Status
	Notes  string
}

func (s *RedeemCodeService) CreateRedeemCodes(ctx context.Context, input CreateRedeemCodesInput) ([]*ent.RedeemCode, error) {
	if input.Count == 0 {
		input.Count = defaultRedeemCodeCount
	}
	if input.Count < 0 || input.Count > maxRedeemCodeBatchSize {
		return nil, fmt.Errorf("redeem code count must be between 1 and %d", maxRedeemCodeBatchSize)
	}
	if input.Type == "" {
		input.Type = redeemcode.TypeBalance
	}
	if input.Type != redeemcode.TypeBalance {
		return nil, fmt.Errorf("redeem code type %q is reserved but not supported yet", input.Type)
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("redeem amount must be positive")
	}
	if input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC()) {
		return nil, fmt.Errorf("redeem code expiration must be in the future")
	}
	if input.BatchID == "" && input.Count > 1 {
		input.BatchID, err = newRedeemBatchID()
		if err != nil {
			return nil, err
		}
	}

	created := make([]*ent.RedeemCode, 0, input.Count)
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		for range input.Count {
			code, err := newRedeemCodeValue(input.Prefix)
			if err != nil {
				return err
			}

			create := client.RedeemCode.Create().
				SetCode(code).
				SetType(input.Type).
				SetAmountMicros(amountMicros).
				SetCurrency(input.Currency).
				SetNotes(strings.TrimSpace(input.Notes)).
				SetBatchID(input.BatchID)
			if input.ExpiresAt != nil {
				create.SetExpiresAt(*input.ExpiresAt)
			}
			if actorID, ok := parsePositiveInt(input.ActorID); ok {
				create.SetCreatedByID(actorID)
			}

			entity, err := create.Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to create redeem code: %w", err)
			}
			created = append(created, entity)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (s *RedeemCodeService) Redeem(ctx context.Context, input RedeemCodeInput) (*ent.RedeemCode, error) {
	codeValue := normalizeRedeemCode(input.Code)
	if codeValue == "" {
		return nil, fmt.Errorf("redeem code is required")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}

	var redeemed *ent.RedeemCode
	now := time.Now().UTC()
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		code, err := client.RedeemCode.Query().
			Where(redeemcode.CodeEQ(codeValue)).
			Only(ctx)
		if ent.IsNotFound(err) {
			return ErrRedeemCodeNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to query redeem code: %w", err)
		}
		if err := validateRedeemableCode(ctx, client, code, now); err != nil {
			return err
		}

		updated, err := client.RedeemCode.Update().
			Where(
				redeemcode.IDEQ(code.ID),
				redeemcode.StatusEQ(redeemcode.StatusActive),
			).
			SetStatus(redeemcode.StatusUsed).
			SetUsedByID(input.UserID).
			SetUsedAt(now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark redeem code used: %w", err)
		}
		if updated == 0 {
			return ErrRedeemCodeUsed
		}

		account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(input.UserID))
		if err != nil {
			return err
		}
		if account.Currency != code.Currency {
			return fmt.Errorf("redeem code currency %s does not match account currency %s", code.Currency, account.Currency)
		}

		createdByType := input.CreatedByType
		if createdByType == "" {
			createdByType = ledgertransaction.CreatedByTypeSystem
		}
		createdByID := input.CreatedByID
		if createdByID == "" {
			createdByID = fmt.Sprint(input.UserID)
		}

		ledgerTx, err := s.ledgerService.Post(ctx, LedgerPostInput{
			BillingAccountID: account.ID,
			Direction:        ledgertransaction.DirectionCredit,
			Amount:           microsToDecimal(code.AmountMicros),
			Currency:         code.Currency,
			Type:             ledgertransaction.TypeRedeemCode,
			IdempotencyKey:   redeemCodeLedgerIdempotencyKey(code.ID),
			ReferenceType:    "redeem_code",
			ReferenceID:      fmt.Sprint(code.ID),
			Memo:             code.Notes,
			CreatedByType:    createdByType,
			CreatedByID:      createdByID,
		})
		if err != nil {
			return err
		}

		redeemed, err = client.RedeemCode.UpdateOneID(code.ID).
			SetLedgerTransactionID(ledgerTx.ID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to attach redeem ledger transaction: %w", err)
		}

		return nil
	})
	if err != nil {
		if errors.Is(err, ErrRedeemCodeExpired) {
			_ = s.markExpiredByCode(ctx, codeValue)
		}
		return nil, err
	}

	return redeemed, nil
}

func (s *RedeemCodeService) AdminCreateAndRedeem(ctx context.Context, input AdminCreateAndRedeemCodeInput) (*ent.RedeemCode, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}

	codes, err := s.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:     1,
		Type:      redeemcode.TypeBalance,
		Amount:    input.Amount,
		Currency:  input.Currency,
		ExpiresAt: input.ExpiresAt,
		Notes:     input.Notes,
		Prefix:    "ADM",
		ActorID:   input.ActorID,
	})
	if err != nil {
		return nil, err
	}

	return s.Redeem(ctx, RedeemCodeInput{
		Code:          codes[0].Code,
		UserID:        input.UserID,
		CreatedByType: ledgertransaction.CreatedByTypeAdmin,
		CreatedByID:   input.ActorID,
	})
}

func (s *RedeemCodeService) UpdateStatus(ctx context.Context, input UpdateRedeemCodeStatusInput) (*ent.RedeemCode, error) {
	if input.CodeID <= 0 {
		return nil, fmt.Errorf("redeem code id is required")
	}
	if input.Status != redeemcode.StatusDisabled && input.Status != redeemcode.StatusExpired && input.Status != redeemcode.StatusActive {
		return nil, fmt.Errorf("unsupported redeem code status %q", input.Status)
	}

	code, err := s.entFromContext(ctx).RedeemCode.Get(ctx, input.CodeID)
	if ent.IsNotFound(err) {
		return nil, ErrRedeemCodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get redeem code: %w", err)
	}
	if code.Status == redeemcode.StatusUsed {
		return nil, ErrRedeemCodeUsed
	}

	update := s.entFromContext(ctx).RedeemCode.UpdateOneID(input.CodeID).
		SetStatus(input.Status)
	if strings.TrimSpace(input.Notes) != "" {
		update.SetNotes(strings.TrimSpace(input.Notes))
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update redeem code status: %w", err)
	}

	return updated, nil
}

func (s *RedeemCodeService) Delete(ctx context.Context, codeID int) error {
	if codeID <= 0 {
		return fmt.Errorf("redeem code id is required")
	}

	code, err := s.entFromContext(ctx).RedeemCode.Get(ctx, codeID)
	if ent.IsNotFound(err) {
		return ErrRedeemCodeNotFound
	}
	if err != nil {
		return fmt.Errorf("failed to get redeem code: %w", err)
	}
	if code.Status == redeemcode.StatusUsed {
		return ErrRedeemCodeUsed
	}

	return s.entFromContext(ctx).RedeemCode.DeleteOneID(codeID).Exec(ctx)
}

func validateRedeemableCode(ctx context.Context, client *ent.Client, code *ent.RedeemCode, now time.Time) error {
	switch code.Status {
	case redeemcode.StatusUsed:
		return ErrRedeemCodeUsed
	case redeemcode.StatusDisabled:
		return ErrRedeemCodeDisabled
	case redeemcode.StatusExpired:
		return ErrRedeemCodeExpired
	case redeemcode.StatusActive:
	default:
		return fmt.Errorf("unsupported redeem code status %q", code.Status)
	}

	if code.Type != redeemcode.TypeBalance {
		return fmt.Errorf("redeem code type %q is reserved but not supported yet", code.Type)
	}
	if code.ExpiresAt != nil && !code.ExpiresAt.After(now) {
		return ErrRedeemCodeExpired
	}

	return nil
}

func (s *RedeemCodeService) markExpiredByCode(ctx context.Context, codeValue string) error {
	_, err := s.entFromContext(ctx).RedeemCode.Update().
		Where(redeemcode.CodeEQ(codeValue), redeemcode.StatusEQ(redeemcode.StatusActive)).
		SetStatus(redeemcode.StatusExpired).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("failed to mark redeem code expired: %w", err)
	}

	return nil
}

func normalizeRedeemCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
}

func newRedeemCodeValue(prefix string) (string, error) {
	random, err := randomHex(8)
	if err != nil {
		return "", err
	}
	prefix = normalizeRedeemCode(prefix)
	if prefix == "" {
		prefix = "AX"
	}

	return prefix + strings.ToUpper(random), nil
}

func newRedeemBatchID() (string, error) {
	random, err := randomHex(6)
	if err != nil {
		return "", err
	}

	return "batch_" + random, nil
}

func redeemCodeLedgerIdempotencyKey(codeID int) string {
	return fmt.Sprintf("redeem_code:%d", codeID)
}

func parsePositiveInt(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	id, err := strconv.Atoi(value)
	if err != nil || id <= 0 {
		return 0, false
	}

	return id, true
}
