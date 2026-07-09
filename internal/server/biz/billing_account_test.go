package biz

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccountbinding"
	"github.com/looplj/axonhub/internal/ent/enttest"
)

func TestBillingAccountServiceGetOrCreateForProject(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_account?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})

	account, err := svc.GetOrCreateForSubject(ctx, ProjectBillingSubject(42))
	require.NoError(t, err)
	require.Equal(t, "project", account.OwnerType.String())
	require.Equal(t, 42, account.OwnerID)
	require.Equal(t, defaultBillingCurrency, account.Currency)
	require.Zero(t, account.BalanceMicros)

	binding, err := client.BillingAccountBinding.Query().
		Where(
			billingaccountbinding.OwnerTypeEQ(billingaccountbinding.OwnerTypeProject),
			billingaccountbinding.OwnerIDEQ(42),
		).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, account.ID, binding.BillingAccountID)
}

func TestBillingAccountServiceGetOrCreateIsIdempotent(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_account_idempotent?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})

	first, err := svc.GetOrCreateForSubject(ctx, ProjectBillingSubject(7))
	require.NoError(t, err)

	second, err := svc.GetOrCreateForSubject(ctx, ProjectBillingSubject(7))
	require.NoError(t, err)
	require.Equal(t, first.ID, second.ID)

	count, err := client.BillingAccount.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestBillingAccountServiceGetOrCreateSerializesConcurrentSubjectCreation(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_account_concurrent?mode=memory&cache=shared&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})

	const workers = 16
	accounts := make(chan *ent.BillingAccount, workers)
	errs := make(chan error, workers)

	var start sync.WaitGroup
	start.Add(1)

	var done sync.WaitGroup
	done.Add(workers)
	for range workers {
		go func() {
			defer done.Done()
			start.Wait()

			account, err := svc.GetOrCreateForSubject(ctx, UserBillingSubject(99))
			if err != nil {
				errs <- err
				return
			}
			accounts <- account
		}()
	}

	start.Done()
	done.Wait()
	close(accounts)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}

	var firstID int
	for account := range accounts {
		if firstID == 0 {
			firstID = account.ID
		}
		require.Equal(t, firstID, account.ID)
	}
	require.NotZero(t, firstID)

	accountCount, err := client.BillingAccount.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, accountCount)

	bindingCount, err := client.BillingAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, bindingCount)
}

func TestBillingAccountServiceGetBySubjectNotFound(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_account_missing?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})

	_, err := svc.GetBySubject(ctx, ProjectBillingSubject(404))
	require.ErrorIs(t, err, ErrBillingAccountNotFound)
}

func TestBillingSubjectValidation(t *testing.T) {
	t.Parallel()

	require.NoError(t, ProjectBillingSubject(1).validate())
	require.NoError(t, UserBillingSubject(1).validate())
	require.Error(t, BillingSubject{Type: "project", ID: 0}.validate())
}

func createBillingAccountForLedgerTest(t *testing.T, client *ent.Client, projectID int) *ent.BillingAccount {
	t.Helper()

	ctx := authz.WithTestBypass(context.Background())
	svc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	account, err := svc.GetOrCreateForSubject(ctx, ProjectBillingSubject(projectID))
	require.NoError(t, err)

	return account
}
