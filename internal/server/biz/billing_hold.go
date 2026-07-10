package biz

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent/dialect/sql"
	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/llm"
)

const defaultHoldTTL = 15 * time.Minute

type BillingHoldServiceParams struct {
	fx.In

	Ent           *ent.Client
	LedgerService *LedgerService
	Config        BillingConfig

	BillingAccountService *BillingAccountService
	PricingService        *PricingService
}

type BillingHoldService struct {
	*AbstractService
	config                BillingConfig
	ledgerService         *LedgerService
	billingAccountService *BillingAccountService
	pricingService        *PricingService
}

func NewBillingHoldService(params BillingHoldServiceParams) *BillingHoldService {
	return &BillingHoldService{
		AbstractService: &AbstractService{db: params.Ent},
		config:          params.Config.normalized(),
		ledgerService:   params.LedgerService,

		billingAccountService: params.BillingAccountService,
		pricingService:        params.PricingService,
	}
}

type CreateBillingHoldInput struct {
	BillingAccountID int
	RequestID        *int
	ProjectID        *int
	UserID           *int
	APIKeyID         *int
	ModelID          string
	Amount           decimal.Decimal
	Currency         string
	IdempotencyKey   string
	ReferenceType    string
	ReferenceID      string
	ExpiresAt        time.Time
}

type CaptureBillingHoldInput struct {
	HoldID         int
	IdempotencyKey string
	Amount         decimal.Decimal
	Currency       string
	UsageLogID     *int
	ReferenceType  string
	ReferenceID    string
	Memo           string
	CreatedByType  ledgertransaction.CreatedByType
	CapturedAt     time.Time
}

type ReleaseBillingHoldInput struct {
	HoldID         int
	IdempotencyKey string
	Reason         string
	ReleasedByType billinghold.ReleasedByType
	ReleasedByID   string
	ReleasedAt     time.Time
	Expire         bool
}

type CreateRequestBillingHoldInput struct {
	Subject   BillingSubject
	RequestID int
	ProjectID int
	APIKeyID  *int
	ModelID   string
	MaxTokens int64
}

func (s *BillingHoldService) CreateRequestHold(ctx context.Context, input CreateRequestBillingHoldInput) (*ent.BillingHold, error) {
	if s.billingAccountService == nil {
		return nil, fmt.Errorf("billing account service is required")
	}
	account, err := s.billingAccountService.GetBySubject(ctx, input.Subject)
	if err != nil {
		return nil, err
	}

	amount := s.estimateRequestHoldAmount(ctx, input.ProjectID, input.ModelID, input.MaxTokens)
	requestID := input.RequestID
	var userID *int
	if input.Subject.Type == BillingSubjectTypeUser {
		userID = &input.Subject.ID
	}

	return s.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		RequestID:        &requestID,
		ProjectID:        &input.ProjectID,
		UserID:           userID,
		APIKeyID:         input.APIKeyID,
		ModelID:          input.ModelID,
		Amount:           roundUsageAmount(amount),
		Currency:         account.Currency,
		IdempotencyKey:   fmt.Sprintf("billing_hold:request:%d", requestID),
		ReferenceType:    "request",
		ReferenceID:      fmt.Sprint(requestID),
		ExpiresAt:        time.Now().UTC().Add(time.Duration(s.config.HoldTTLSeconds) * time.Second),
	})
}

func (s *BillingHoldService) CreateHold(ctx context.Context, input CreateBillingHoldInput) (*ent.BillingHold, error) {
	if input.BillingAccountID <= 0 {
		return nil, fmt.Errorf("billing account id is required")
	}
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency key is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.ExpiresAt.IsZero() {
		input.ExpiresAt = time.Now().UTC().Add(defaultHoldTTL)
	}
	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("hold amount must be positive")
	}

	var hold *ent.BillingHold
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		existing, err := client.BillingHold.Query().
			Where(billinghold.IdempotencyKeyEQ(input.IdempotencyKey)).
			Only(ctx)
		if err == nil {
			hold = existing
			return nil
		}
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query billing hold idempotency key: %w", err)
		}

		account, err := client.BillingAccount.Get(ctx, input.BillingAccountID)
		if err != nil {
			return fmt.Errorf("failed to get billing account: %w", err)
		}
		if account.Status == billingaccount.StatusFrozen {
			return ErrBillingAccountFrozen
		}
		if account.Status == billingaccount.StatusClosed {
			return ErrBillingAccountClosed
		}
		if account.Currency != input.Currency {
			return fmt.Errorf("hold currency %s does not match account currency %s", input.Currency, account.Currency)
		}

		updated, err := client.BillingAccount.Update().
			Where(
				billingaccount.IDEQ(account.ID),
				billingAccountAvailableGTE(amountMicros),
			).
			AddHeldBalanceMicros(amountMicros).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to reserve billing hold: %w", err)
		}
		if updated == 0 {
			return ErrInsufficientBalance
		}

		create := client.BillingHold.Create().
			SetBillingAccountID(account.ID).
			SetAmountMicros(amountMicros).
			SetCurrency(input.Currency).
			SetIdempotencyKey(input.IdempotencyKey).
			SetReferenceType(input.ReferenceType).
			SetReferenceID(input.ReferenceID).
			SetExpiresAt(input.ExpiresAt)
		if input.RequestID != nil {
			create.SetRequestID(*input.RequestID)
		}
		if input.ProjectID != nil {
			create.SetProjectID(*input.ProjectID)
		}
		if input.UserID != nil {
			create.SetUserID(*input.UserID)
		}
		if input.APIKeyID != nil {
			create.SetAPIKeyID(*input.APIKeyID)
		}
		if input.ModelID != "" {
			create.SetModelID(input.ModelID)
		}

		created, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create billing hold: %w", err)
		}

		hold = created
		return nil
	})
	if ent.IsConstraintError(err) {
		return s.entFromContext(ctx).BillingHold.Query().
			Where(billinghold.IdempotencyKeyEQ(input.IdempotencyKey)).
			Only(ctx)
	}
	if err != nil {
		return nil, err
	}

	return hold, nil
}

func (s *BillingHoldService) CaptureHold(ctx context.Context, input CaptureBillingHoldInput) (*ent.BillingHold, error) {
	if input.HoldID <= 0 && input.IdempotencyKey == "" {
		return nil, fmt.Errorf("hold id or idempotency key is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.CapturedAt.IsZero() {
		input.CapturedAt = time.Now().UTC()
	}
	if input.CreatedByType == "" {
		input.CreatedByType = ledgertransaction.CreatedByTypeSystem
	}
	actualMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}

	var captured *ent.BillingHold
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		hold, err := s.getHoldForMutation(ctx, input.HoldID, input.IdempotencyKey)
		if err != nil {
			return err
		}
		if hold.Status != billinghold.StatusHeld {
			captured = hold
			return nil
		}
		if hold.Currency != input.Currency {
			return fmt.Errorf("capture currency %s does not match hold currency %s", input.Currency, hold.Currency)
		}

		overageMicros := actualMicros - hold.AmountMicros
		if overageMicros > 0 {
			account, err := client.BillingAccount.Get(ctx, hold.BillingAccountID)
			if err != nil {
				return fmt.Errorf("failed to get billing account: %w", err)
			}
			if account.BalanceMicros+account.CreditLimitMicros-account.HeldBalanceMicros < overageMicros {
				return ErrInsufficientBalance
			}
		}

		update := client.BillingHold.Update().
			Where(billinghold.IDEQ(hold.ID), billinghold.StatusEQ(billinghold.StatusHeld)).
			SetStatus(billinghold.StatusCaptured).
			SetCapturedAmountMicros(actualMicros).
			SetCapturedAt(input.CapturedAt)
		if input.UsageLogID != nil {
			update.SetUsageLogID(*input.UsageLogID)
		}
		updated, err := update.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark billing hold captured: %w", err)
		}
		if updated == 0 {
			reloaded, err := client.BillingHold.Get(ctx, hold.ID)
			if err != nil {
				return err
			}
			captured = reloaded
			return nil
		}

		if _, err := client.BillingAccount.UpdateOneID(hold.BillingAccountID).
			AddHeldBalanceMicros(-hold.AmountMicros).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to release captured billing hold amount: %w", err)
		}

		var ledgerTx *ent.LedgerTransaction
		if actualMicros > 0 {
			ledgerInput := LedgerPostInput{
				BillingAccountID: hold.BillingAccountID,
				Direction:        ledgertransaction.DirectionDebit,
				Amount:           input.Amount,
				Currency:         input.Currency,
				Type:             ledgertransaction.TypeUsageCharge,
				IdempotencyKey:   "billing_hold_capture:" + hold.IdempotencyKey,
				ReferenceType:    input.ReferenceType,
				ReferenceID:      input.ReferenceID,
				Memo:             input.Memo,
				CreatedByType:    input.CreatedByType,
			}
			ledgerTx, err = s.ledgerService.Post(ctx, ledgerInput)
			if err != nil {
				return err
			}

			_, err = client.BillingHold.UpdateOneID(hold.ID).
				SetCapturedLedgerTransactionID(ledgerTx.ID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to attach billing hold ledger transaction: %w", err)
			}
		}

		reloaded, err := client.BillingHold.Get(ctx, hold.ID)
		if err != nil {
			return err
		}
		captured = reloaded
		return nil
	})
	if err != nil {
		return nil, err
	}

	return captured, nil
}

func (s *BillingHoldService) ReleaseHold(ctx context.Context, input ReleaseBillingHoldInput) (*ent.BillingHold, error) {
	if input.HoldID <= 0 && input.IdempotencyKey == "" {
		return nil, fmt.Errorf("hold id or idempotency key is required")
	}
	if input.Reason == "" {
		input.Reason = "released"
	}
	if input.ReleasedAt.IsZero() {
		input.ReleasedAt = time.Now().UTC()
	}
	if input.ReleasedByType == "" {
		input.ReleasedByType = billinghold.ReleasedByTypeSystem
	}

	var released *ent.BillingHold
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		hold, err := s.getHoldForMutation(ctx, input.HoldID, input.IdempotencyKey)
		if err != nil {
			return err
		}
		if hold.Status != billinghold.StatusHeld {
			released = hold
			return nil
		}

		nextStatus := billinghold.StatusReleased
		if input.Expire {
			nextStatus = billinghold.StatusExpired
		}
		updated, err := client.BillingHold.Update().
			Where(billinghold.IDEQ(hold.ID), billinghold.StatusEQ(billinghold.StatusHeld)).
			SetStatus(nextStatus).
			SetReleaseReason(input.Reason).
			SetReleasedByType(input.ReleasedByType).
			SetReleasedByID(input.ReleasedByID).
			SetReleasedAt(input.ReleasedAt).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to release billing hold: %w", err)
		}
		if updated == 0 {
			reloaded, err := client.BillingHold.Get(ctx, hold.ID)
			if err != nil {
				return err
			}
			released = reloaded
			return nil
		}

		if _, err := client.BillingAccount.UpdateOneID(hold.BillingAccountID).
			AddHeldBalanceMicros(-hold.AmountMicros).
			Save(ctx); err != nil {
			return fmt.Errorf("failed to release billing account held amount: %w", err)
		}

		reloaded, err := client.BillingHold.Get(ctx, hold.ID)
		if err != nil {
			return err
		}
		released = reloaded
		return nil
	})
	if err != nil {
		return nil, err
	}

	return released, nil
}

func (s *BillingHoldService) ExpireDueHolds(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}

	holds, err := s.entFromContext(ctx).BillingHold.Query().
		Where(
			billinghold.StatusEQ(billinghold.StatusHeld),
			billinghold.ExpiresAtLTE(now),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired billing holds: %w", err)
	}

	var expired int
	for _, hold := range holds {
		released, err := s.ReleaseHold(ctx, ReleaseBillingHoldInput{
			HoldID:         hold.ID,
			Reason:         "expired",
			ReleasedByType: billinghold.ReleasedByTypeSystem,
			ReleasedAt:     now,
			Expire:         true,
		})
		if err != nil {
			return expired, err
		}
		if released.Status == billinghold.StatusExpired {
			expired++
		}
	}

	return expired, nil
}

func (s *BillingHoldService) FindHeldHoldForUsageLog(ctx context.Context, usageLog *ent.UsageLog) (*ent.BillingHold, error) {
	hold, err := s.entFromContext(ctx).BillingHold.Query().
		Where(
			billinghold.RequestIDEQ(usageLog.RequestID),
			billinghold.StatusEQ(billinghold.StatusHeld),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query billing hold for usage log: %w", err)
	}

	return hold, nil
}

func (s *BillingHoldService) getHoldForMutation(ctx context.Context, holdID int, idempotencyKey string) (*ent.BillingHold, error) {
	if holdID > 0 {
		hold, err := s.entFromContext(ctx).BillingHold.Get(ctx, holdID)
		if err != nil {
			return nil, fmt.Errorf("failed to get billing hold: %w", err)
		}
		return hold, nil
	}

	hold, err := s.entFromContext(ctx).BillingHold.Query().
		Where(billinghold.IdempotencyKeyEQ(idempotencyKey)).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get billing hold by idempotency key: %w", err)
	}
	return hold, nil
}

func billingAccountAvailableGTE(amountMicros int64) predicate.BillingAccount {
	return predicate.BillingAccount(func(s *sql.Selector) {
		s.Where(sql.P(func(b *sql.Builder) {
			b.Ident(s.C(billingaccount.FieldBalanceMicros)).
				WriteOp(sql.OpAdd).
				Ident(s.C(billingaccount.FieldCreditLimitMicros)).
				WriteOp(sql.OpSub).
				Ident(s.C(billingaccount.FieldHeldBalanceMicros)).
				WriteOp(sql.OpGTE).
				Arg(amountMicros)
		}))
	})
}

func (s *BillingHoldService) estimateRequestHoldAmount(ctx context.Context, projectID int, modelID string, maxTokens int64) decimal.Decimal {
	if s.pricingService != nil && maxTokens > 0 {
		priceRule, err := s.pricingService.FindSellPrice(ctx, projectID, modelID)
		if err == nil {
			_, total := ComputeUsageCost(&llm.Usage{CompletionTokens: maxTokens, TotalTokens: maxTokens}, priceRule.Price)
			if total.IsPositive() {
				return total
			}
		}
	}

	return s.config.HoldDefaultAmount
}
