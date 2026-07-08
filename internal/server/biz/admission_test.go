package biz

import (
	"context"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
)

func newAdmissionTestServices(t *testing.T, name string, cfg BillingConfig) (*AdmissionService, *BillingAccountService, *LedgerService, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	admissionSvc := NewAdmissionService(AdmissionServiceParams{
		Config:                cfg,
		BillingAccountService: accountSvc,
	})

	return admissionSvc, accountSvc, ledgerSvc, ctx
}

func TestAdmissionDisabledDoesNotRequireBillingAccount(t *testing.T) {
	t.Parallel()

	svc, _, _, ctx := newAdmissionTestServices(t, "admission_disabled", BillingConfig{Mode: AdmissionModeDisabled})

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(999)})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, AdmissionModeDisabled, decision.Mode)
	require.Equal(t, AdmissionCodeBillingDisabled, decision.Code)
}

func TestAdmissionWarnAllowsMissingBillingAccount(t *testing.T) {
	t.Parallel()

	svc, _, _, ctx := newAdmissionTestServices(t, "admission_warn_missing", BillingConfig{Mode: AdmissionModeWarn})

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(999)})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAccountNotFound, decision.Code)
	require.Equal(t, "billing account not found", decision.Reason)
}

func TestAdmissionEnforceRejectsMissingBillingAccount(t *testing.T) {
	t.Parallel()

	svc, _, _, ctx := newAdmissionTestServices(t, "admission_enforce_missing", BillingConfig{Mode: AdmissionModeEnforce})

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(999)})
	require.ErrorIs(t, err, ErrBillingAccountNotFound)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAccountNotFound, decision.Code)
}

func TestAdmissionEnforceRejectsZeroBalanceByDefault(t *testing.T) {
	t.Parallel()

	svc, accountSvc, _, ctx := newAdmissionTestServices(t, "admission_zero_balance", BillingConfig{Mode: AdmissionModeEnforce})
	_, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeInsufficientBalance, decision.Code)
}

func TestAdmissionEnforceAllowsPositiveBalance(t *testing.T) {
	t.Parallel()

	svc, accountSvc, ledgerSvc, ctx := newAdmissionTestServices(t, "admission_positive_balance", BillingConfig{Mode: AdmissionModeEnforce})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "recharge")
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAllowed, decision.Code)
}

func TestAdmissionEnforceAllowsCreditLimit(t *testing.T) {
	t.Parallel()

	svc, accountSvc, _, ctx := newAdmissionTestServices(t, "admission_credit_limit", BillingConfig{Mode: AdmissionModeEnforce})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	_, err = accountSvc.entFromContext(ctx).BillingAccount.UpdateOneID(account.ID).SetCreditLimitMicros(1_000_000).Save(ctx)
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAllowed, decision.Code)
}

func TestAdmissionEnforceSubtractsHeldBalance(t *testing.T) {
	t.Parallel()

	svc, accountSvc, ledgerSvc, ctx := newAdmissionTestServices(t, "admission_held_balance", BillingConfig{Mode: AdmissionModeEnforce})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "recharge-held")
	require.NoError(t, err)
	_, err = accountSvc.entFromContext(ctx).BillingAccount.UpdateOneID(account.ID).SetHeldBalanceMicros(1_000_000).Save(ctx)
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeInsufficientBalance, decision.Code)
}

func TestAdmissionRejectsFrozenAccount(t *testing.T) {
	t.Parallel()

	svc, accountSvc, _, ctx := newAdmissionTestServices(t, "admission_frozen", BillingConfig{Mode: AdmissionModeEnforce})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	_, err = accountSvc.entFromContext(ctx).BillingAccount.UpdateOneID(account.ID).SetStatus(billingaccount.StatusFrozen).Save(ctx)
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.ErrorIs(t, err, ErrBillingAccountFrozen)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAccountFrozen, decision.Code)
}

func TestAdmissionRejectsClosedAccount(t *testing.T) {
	t.Parallel()

	svc, accountSvc, _, ctx := newAdmissionTestServices(t, "admission_closed", BillingConfig{Mode: AdmissionModeEnforce})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	_, err = accountSvc.entFromContext(ctx).BillingAccount.UpdateOneID(account.ID).SetStatus(billingaccount.StatusClosed).Save(ctx)
	require.NoError(t, err)

	decision, err := svc.Check(ctx, AdmissionCheckInput{Subject: ProjectBillingSubject(1)})
	require.ErrorIs(t, err, ErrBillingAccountClosed)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAccountClosed, decision.Code)
}
