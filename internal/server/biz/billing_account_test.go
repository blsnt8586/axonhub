package biz

import (
	"context"
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
	require.Error(t, BillingSubject{Type: "user", ID: 1}.validate())
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
