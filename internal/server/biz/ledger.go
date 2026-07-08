package biz

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/ledgerentry"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
)

type LedgerServiceParams struct {
	fx.In

	Ent *ent.Client
}

type LedgerService struct {
	*AbstractService
}

func NewLedgerService(params LedgerServiceParams) *LedgerService {
	return &LedgerService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

type LedgerPostInput struct {
	BillingAccountID int
	Direction        ledgertransaction.Direction
	Amount           decimal.Decimal
	Currency         string
	Type             ledgertransaction.Type
	IdempotencyKey   string
	ReferenceType    string
	ReferenceID      string
	Memo             string
	CreatedByType    ledgertransaction.CreatedByType
	CreatedByID      string
}

func (s *LedgerService) Post(ctx context.Context, input LedgerPostInput) (*ent.LedgerTransaction, error) {
	if input.IdempotencyKey == "" {
		return nil, fmt.Errorf("idempotency key is required")
	}
	if input.BillingAccountID <= 0 {
		return nil, fmt.Errorf("billing account id is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.CreatedByType == "" {
		input.CreatedByType = ledgertransaction.CreatedByTypeSystem
	}

	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	var posted *ent.LedgerTransaction
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		existing, err := client.LedgerTransaction.Query().
			Where(ledgertransaction.IdempotencyKeyEQ(input.IdempotencyKey)).
			Only(ctx)
		if err == nil {
			posted = existing
			return nil
		}
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query ledger idempotency key: %w", err)
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
			return fmt.Errorf("ledger currency %s does not match account currency %s", input.Currency, account.Currency)
		}

		deltaMicros := amountMicros
		if input.Direction == ledgertransaction.DirectionDebit {
			deltaMicros = -amountMicros
			if account.BalanceMicros+account.CreditLimitMicros < amountMicros {
				return ErrInsufficientBalance
			}
		}

		created, err := client.LedgerTransaction.Create().
			SetBillingAccountID(account.ID).
			SetDirection(input.Direction).
			SetAmountMicros(amountMicros).
			SetCurrency(input.Currency).
			SetType(input.Type).
			SetIdempotencyKey(input.IdempotencyKey).
			SetReferenceType(input.ReferenceType).
			SetReferenceID(input.ReferenceID).
			SetMemo(input.Memo).
			SetCreatedByType(input.CreatedByType).
			SetCreatedByID(input.CreatedByID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create ledger transaction: %w", err)
		}

		_, err = client.LedgerEntry.Create().
			SetLedgerTransactionID(created.ID).
			SetAccountSide(ledgerentry.AccountSideCustomerBalance).
			SetDirection(toLedgerEntryDirection(input.Direction)).
			SetAmountMicros(amountMicros).
			SetCurrency(input.Currency).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create ledger entry: %w", err)
		}

		_, err = client.BillingAccount.UpdateOneID(account.ID).
			AddBalanceMicros(deltaMicros).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update billing account balance: %w", err)
		}

		posted = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	return posted, nil
}

func toLedgerEntryDirection(direction ledgertransaction.Direction) ledgerentry.Direction {
	if direction == ledgertransaction.DirectionCredit {
		return ledgerentry.DirectionCredit
	}

	return ledgerentry.DirectionDebit
}

func (s *LedgerService) Credit(ctx context.Context, accountID int, amount decimal.Decimal, txType ledgertransaction.Type, idempotencyKey string) (*ent.LedgerTransaction, error) {
	return s.Post(ctx, LedgerPostInput{
		BillingAccountID: accountID,
		Direction:        ledgertransaction.DirectionCredit,
		Amount:           amount,
		Type:             txType,
		IdempotencyKey:   idempotencyKey,
	})
}

func (s *LedgerService) Debit(ctx context.Context, accountID int, amount decimal.Decimal, txType ledgertransaction.Type, idempotencyKey string) (*ent.LedgerTransaction, error) {
	return s.Post(ctx, LedgerPostInput{
		BillingAccountID: accountID,
		Direction:        ledgertransaction.DirectionDebit,
		Amount:           amount,
		Type:             txType,
		IdempotencyKey:   idempotencyKey,
	})
}
