package biz

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
)

func TestLedgerServiceCreditAndDebit(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_credit_debit?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 1)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	credit, err := svc.Credit(ctx, account.ID, decimal.RequireFromString("10.500001"), ledgertransaction.TypePaymentRecharge, "credit-1")
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.DirectionCredit, credit.Direction)

	debit, err := svc.Debit(ctx, account.ID, decimal.RequireFromString("3.25"), ledgertransaction.TypeUsageCharge, "debit-1")
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.DirectionDebit, debit.Direction)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(7250001), reloaded.BalanceMicros)

	entries, err := client.LedgerEntry.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, entries)
}

func TestLedgerServiceIdempotencyDoesNotMutateBalanceTwice(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_idempotency?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 2)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	first, err := svc.Credit(ctx, account.ID, decimal.RequireFromString("5"), ledgertransaction.TypePaymentRecharge, "same-key")
	require.NoError(t, err)
	second, err := svc.Credit(ctx, account.ID, decimal.RequireFromString("5"), ledgertransaction.TypePaymentRecharge, "same-key")
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(5_000_000), reloaded.BalanceMicros)

	count, err := client.LedgerTransaction.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestLedgerServiceRejectsInsufficientBalance(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_insufficient?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 3)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	_, err := svc.Debit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypeUsageCharge, "debit-too-much")
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestLedgerServiceAllowsDebitWithinCreditLimit(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_credit_limit?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 4)
	_, err := client.BillingAccount.UpdateOneID(account.ID).SetCreditLimitMicros(2_000_000).Save(ctx)
	require.NoError(t, err)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	_, err = svc.Debit(ctx, account.ID, decimal.RequireFromString("1.5"), ledgertransaction.TypeUsageCharge, "debit-credit-limit")
	require.NoError(t, err)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(-1_500_000), reloaded.BalanceMicros)
}

func TestLedgerServiceRejectsFrozenAccount(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_frozen?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 5)
	_, err := client.BillingAccount.UpdateOneID(account.ID).SetStatus(billingaccount.StatusFrozen).Save(ctx)
	require.NoError(t, err)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	_, err = svc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "credit-frozen")
	require.ErrorIs(t, err, ErrBillingAccountFrozen)
}

func TestLedgerServiceRequiresIdempotencyKey(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:ledger_requires_idempotency?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	account := createBillingAccountForLedgerTest(t, client, 6)
	svc := NewLedgerService(LedgerServiceParams{Ent: client})

	_, err := svc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "")
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrInsufficientBalance))
}
