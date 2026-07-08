package biz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/redeemcode"
)

func TestRedeemCodeServiceRedeemCreditsLedgerOnce(t *testing.T) {
	t.Parallel()

	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_credit_once")

	codes, err := svc.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:    1,
		Amount:   decimal.RequireFromString("12.345678"),
		Currency: "CNY",
		Notes:    "campaign grant",
		ActorID:  "1",
	})
	require.NoError(t, err)
	require.Len(t, codes, 1)

	redeemed, err := svc.Redeem(ctx, RedeemCodeInput{Code: codes[0].Code, UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, redeemcode.StatusUsed, redeemed.Status)
	require.Equal(t, user.ID, *redeemed.UsedByID)
	require.NotNil(t, redeemed.LedgerTransactionID)

	_, err = svc.Redeem(ctx, RedeemCodeInput{Code: codes[0].Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeUsed)

	account, err := svc.billingAccountService.GetBySubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	require.Equal(t, int64(12_345_678), account.BalanceMicros)

	ledgerCount, err := client.LedgerTransaction.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerCount)
}

func TestRedeemCodeServiceRejectsUsedDisabledExpired(t *testing.T) {
	t.Parallel()

	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_reject_statuses")
	expiredAt := time.Now().UTC().Add(-time.Minute)

	used := client.RedeemCode.Create().
		SetCode("USED001").
		SetType(redeemcode.TypeBalance).
		SetStatus(redeemcode.StatusUsed).
		SetAmountMicros(1_000_000).
		SetCurrency("CNY").
		SetUsedByID(user.ID).
		SetUsedAt(time.Now().UTC()).
		SaveX(ctx)
	disabled := client.RedeemCode.Create().
		SetCode("DISABLED001").
		SetType(redeemcode.TypeBalance).
		SetStatus(redeemcode.StatusDisabled).
		SetAmountMicros(1_000_000).
		SetCurrency("CNY").
		SaveX(ctx)
	expiredStatus := client.RedeemCode.Create().
		SetCode("EXPIRED001").
		SetType(redeemcode.TypeBalance).
		SetStatus(redeemcode.StatusExpired).
		SetAmountMicros(1_000_000).
		SetCurrency("CNY").
		SaveX(ctx)
	expiredByTime := client.RedeemCode.Create().
		SetCode("TIMEEXPIRED001").
		SetType(redeemcode.TypeBalance).
		SetStatus(redeemcode.StatusActive).
		SetAmountMicros(1_000_000).
		SetCurrency("CNY").
		SetExpiresAt(expiredAt).
		SaveX(ctx)

	_, err := svc.Redeem(ctx, RedeemCodeInput{Code: used.Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeUsed)
	_, err = svc.Redeem(ctx, RedeemCodeInput{Code: disabled.Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeDisabled)
	_, err = svc.Redeem(ctx, RedeemCodeInput{Code: expiredStatus.Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeExpired)
	_, err = svc.Redeem(ctx, RedeemCodeInput{Code: expiredByTime.Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeExpired)

	reloaded, err := client.RedeemCode.Get(ctx, expiredByTime.ID)
	require.NoError(t, err)
	require.Equal(t, redeemcode.StatusExpired, reloaded.Status)

	account, err := svc.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	require.Zero(t, account.BalanceMicros)
}

func TestRedeemCodeServiceConcurrentRedeemCreditsOnce(t *testing.T) {
	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_concurrent_once")

	codes, err := svc.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:    1,
		Amount:   decimal.RequireFromString("5"),
		Currency: "CNY",
	})
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := range 10 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Redeem(ctx, RedeemCodeInput{Code: codes[0].Code, UserID: user.ID})
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	var success int
	var usedErrors int
	for err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, ErrRedeemCodeUsed):
			usedErrors++
		default:
			require.NoError(t, err)
		}
	}
	require.Equal(t, 1, success)
	require.Equal(t, 9, usedErrors)

	account, err := svc.billingAccountService.GetBySubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	require.Equal(t, int64(5_000_000), account.BalanceMicros)

	ledgerCount, err := client.LedgerTransaction.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerCount)
}

func TestRedeemCodeServiceCreateAndRedeemAuditsAdmin(t *testing.T) {
	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_admin_create_and_redeem")
	admin := client.User.Create().
		SetEmail("redeem-admin@example.com").
		SetPassword("pw").
		SetIsOwner(true).
		SaveX(ctx)

	redeemed, err := svc.AdminCreateAndRedeem(ctx, AdminCreateAndRedeemCodeInput{
		UserID:   user.ID,
		Amount:   decimal.RequireFromString("9.5"),
		Currency: "CNY",
		Notes:    "admin compensation",
		ActorID:  fmt.Sprint(admin.ID),
	})
	require.NoError(t, err)
	require.Equal(t, redeemcode.StatusUsed, redeemed.Status)
	require.Equal(t, admin.ID, *redeemed.CreatedByID)
	require.Equal(t, user.ID, *redeemed.UsedByID)

	tx, err := client.LedgerTransaction.Get(ctx, *redeemed.LedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.CreatedByTypeAdmin, tx.CreatedByType)
	require.Equal(t, fmt.Sprint(admin.ID), tx.CreatedByID)
	require.Equal(t, "admin compensation", tx.Memo)
}

func TestRedeemCodeServiceLedgerReferenceLinksRedeemCode(t *testing.T) {
	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_ledger_reference")

	codes, err := svc.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:    1,
		Amount:   decimal.RequireFromString("3"),
		Currency: "CNY",
	})
	require.NoError(t, err)

	redeemed, err := svc.Redeem(ctx, RedeemCodeInput{Code: codes[0].Code, UserID: user.ID})
	require.NoError(t, err)

	tx, err := client.LedgerTransaction.Get(ctx, *redeemed.LedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, ledgertransaction.TypeRedeemCode, tx.Type)
	require.Equal(t, "redeem_code", tx.ReferenceType)
	require.Equal(t, fmt.Sprint(redeemed.ID), tx.ReferenceID)
	require.Equal(t, redeemCodeLedgerIdempotencyKey(redeemed.ID), tx.IdempotencyKey)
	require.Equal(t, ledgertransaction.DirectionCredit, tx.Direction)

	linked, err := client.RedeemCode.Query().
		Where(redeemcode.LedgerTransactionIDEQ(tx.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, redeemed.ID, linked.ID)
}

func TestRedeemCodeServiceBatchGenerateDisableDelete(t *testing.T) {
	client, ctx, svc, user := newRedeemCodeTestService(t, "redeem_code_batch_admin_ops")

	codes, err := svc.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:    3,
		Amount:   decimal.RequireFromString("1"),
		Currency: "CNY",
		Prefix:   "promo",
		Notes:    "batch grant",
	})
	require.NoError(t, err)
	require.Len(t, codes, 3)
	require.NotEmpty(t, codes[0].BatchID)
	for _, code := range codes {
		require.Contains(t, code.Code, "PROMO")
		require.Equal(t, codes[0].BatchID, code.BatchID)
	}

	disabled, err := svc.UpdateStatus(ctx, UpdateRedeemCodeStatusInput{
		CodeID: codes[0].ID,
		Status: redeemcode.StatusDisabled,
		Notes:  "campaign stopped",
	})
	require.NoError(t, err)
	require.Equal(t, redeemcode.StatusDisabled, disabled.Status)
	require.Equal(t, "campaign stopped", disabled.Notes)

	_, err = svc.Redeem(ctx, RedeemCodeInput{Code: disabled.Code, UserID: user.ID})
	require.ErrorIs(t, err, ErrRedeemCodeDisabled)

	err = svc.Delete(ctx, codes[1].ID)
	require.NoError(t, err)
	_, err = client.RedeemCode.Get(ctx, codes[1].ID)
	require.True(t, ent.IsNotFound(err))
}

func newRedeemCodeTestService(t *testing.T, name string) (*ent.Client, context.Context, *RedeemCodeService, *ent.User) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&cache=shared&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	user := client.User.Create().
		SetEmail(name + "@example.com").
		SetPassword("pw").
		SaveX(ctx)

	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	svc := NewRedeemCodeService(RedeemCodeServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
	})

	return client, ctx, svc, user
}
