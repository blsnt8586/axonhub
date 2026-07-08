package biz

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
)

func TestBillingHoldServiceCreateReserveAndIdempotency(t *testing.T) {
	t.Parallel()

	client, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_create")
	hold, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("2.5"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-create",
		ExpiresAt:        time.Now().UTC().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, billinghold.StatusHeld, hold.Status)
	require.Equal(t, int64(2_500_000), hold.AmountMicros)

	same, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("2.5"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-create",
		ExpiresAt:        time.Now().UTC().Add(time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, hold.ID, same.ID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2_500_000), reloaded.HeldBalanceMicros)
}

func TestBillingHoldServiceRejectsReserveBeyondAvailable(t *testing.T) {
	t.Parallel()

	_, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_insufficient")
	_, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("10.000001"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-too-large",
	})
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestBillingHoldServiceReleaseIsIdempotent(t *testing.T) {
	t.Parallel()

	client, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_release")
	hold, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("3"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-release",
	})
	require.NoError(t, err)

	released, err := svc.ReleaseHold(ctx, ReleaseBillingHoldInput{
		HoldID: hold.ID,
		Reason: "upstream failure",
	})
	require.NoError(t, err)
	require.Equal(t, billinghold.StatusReleased, released.Status)

	releasedAgain, err := svc.ReleaseHold(ctx, ReleaseBillingHoldInput{HoldID: hold.ID, Reason: "retry"})
	require.NoError(t, err)
	require.Equal(t, released.ID, releasedAgain.ID)
	require.Equal(t, billinghold.StatusReleased, releasedAgain.Status)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Zero(t, reloaded.HeldBalanceMicros)
}

func TestBillingHoldServiceCapturePostsLedgerOnceAndReleasesHold(t *testing.T) {
	t.Parallel()

	client, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_capture")
	hold, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("3"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-capture",
	})
	require.NoError(t, err)

	captured, err := svc.CaptureHold(ctx, CaptureBillingHoldInput{
		HoldID:        hold.ID,
		Amount:        decimal.RequireFromString("1.25"),
		Currency:      "CNY",
		ReferenceType: "test",
		ReferenceID:   "capture",
		Memo:          "capture test",
	})
	require.NoError(t, err)
	require.Equal(t, billinghold.StatusCaptured, captured.Status)
	require.Equal(t, int64(1_250_000), captured.CapturedAmountMicros)
	require.NotZero(t, captured.CapturedLedgerTransactionID)

	capturedAgain, err := svc.CaptureHold(ctx, CaptureBillingHoldInput{
		HoldID:   hold.ID,
		Amount:   decimal.RequireFromString("1.25"),
		Currency: "CNY",
	})
	require.NoError(t, err)
	require.Equal(t, captured.ID, capturedAgain.ID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Zero(t, reloaded.HeldBalanceMicros)
	require.Equal(t, int64(8_750_000), reloaded.BalanceMicros)

	count, err := client.LedgerTransaction.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count, "initial credit plus capture debit")
}

func TestBillingHoldServiceExpireDueHoldsIsRepeatable(t *testing.T) {
	t.Parallel()

	client, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_expire")
	now := time.Now().UTC()
	_, err := svc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("2"),
		Currency:         "CNY",
		IdempotencyKey:   "hold-expire",
		ExpiresAt:        now.Add(-time.Minute),
	})
	require.NoError(t, err)

	expired, err := svc.ExpireDueHolds(ctx, now, 10)
	require.NoError(t, err)
	require.Equal(t, 1, expired)

	expiredAgain, err := svc.ExpireDueHolds(ctx, now, 10)
	require.NoError(t, err)
	require.Zero(t, expiredAgain)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Zero(t, reloaded.HeldBalanceMicros)
}

func TestBillingHoldServiceConcurrentReserveCannotOverspend(t *testing.T) {
	client, ctx, svc, account := newBillingHoldTestService(t, "billing_hold_concurrent")

	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for i := range 5 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.CreateHold(ctx, CreateBillingHoldInput{
				BillingAccountID: account.ID,
				Amount:           decimal.RequireFromString("4"),
				Currency:         "CNY",
				IdempotencyKey:   fmt.Sprintf("hold-concurrent-%d", i),
			})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	var success int
	for err := range errs {
		if err == nil {
			success++
		}
	}
	require.LessOrEqual(t, success, 2)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.LessOrEqual(t, reloaded.HeldBalanceMicros, int64(8_000_000))
}

func newBillingHoldTestService(t *testing.T, name string) (*ent.Client, context.Context, *BillingHoldService, *ent.BillingAccount) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	account, err := accountSvc.GetOrCreateForSubject(ctx, UserBillingSubject(1))
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)
	svc := NewBillingHoldService(BillingHoldServiceParams{Ent: client, LedgerService: ledgerSvc})

	return client, ctx, svc, account
}
