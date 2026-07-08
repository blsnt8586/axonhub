package biz

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	entuser "github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func TestAPIKeyCommercialLimitServiceDailyAndMonthlyWindowsResetDeterministically(t *testing.T) {
	t.Parallel()

	client, ctx, svc, apiKey, account := newAPIKeyCommercialLimitTestService(t, "commercial_limit_windows")
	now := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	dailyBudget := int64(2_000_000)
	monthlyBudget := int64(3_000_000)
	apiKey, err := client.APIKey.UpdateOneID(apiKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:             true,
			Currency:            "CNY",
			DailyBudgetMicros:   &dailyBudget,
			MonthlyBudgetMicros: &monthlyBudget,
		}).
		Save(ctx)
	require.NoError(t, err)

	seedCommercialChargedRecord(t, client, ctx, account, apiKey, 2_000_000, "CNY", now.AddDate(0, 0, -1))
	seedCommercialChargedRecord(t, client, ctx, account, apiKey, 1_000_000, "CNY", now.Add(-time.Hour))

	usage, err := svc.Usage(ctx, apiKey, "CNY", now)
	require.NoError(t, err)
	require.Equal(t, int64(1_000_000), usage.Daily.SpentMicros)
	require.Equal(t, int64(3_000_000), usage.Monthly.SpentMicros)
	require.Equal(t, int64(1_000_000), *usage.Daily.RemainingMicros)
	require.Zero(t, *usage.Monthly.RemainingMicros)
	require.False(t, usage.Daily.Exceeded)
	require.True(t, usage.Monthly.Exceeded)

	check, err := svc.Check(ctx, APIKeyCommercialLimitCheckInput{
		APIKey:                apiKey,
		Currency:              "CNY",
		EstimatedChargeMicros: 1,
		Now:                   now,
	})
	require.ErrorIs(t, err, ErrAPIKeyCommercialLimitExceeded)
	require.False(t, check.Allowed)
	require.Contains(t, check.Message, "monthly budget exceeded")
}

func TestAdmissionRejectsWhenWalletSufficientButAPIKeyBudgetExceeded(t *testing.T) {
	t.Parallel()

	client, ctx, limitSvc, apiKey, account := newAPIKeyCommercialLimitTestService(t, "admission_key_budget")
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err := ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "credit")
	require.NoError(t, err)
	budget := int64(1_000_000)
	apiKey, err = client.APIKey.UpdateOneID(apiKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:           true,
			Currency:          "CNY",
			TotalBudgetMicros: &budget,
		}).
		Save(ctx)
	require.NoError(t, err)
	seedCommercialChargedRecord(t, client, ctx, account, apiKey, 1_000_000, "CNY", time.Now().UTC())

	admissionSvc := NewAdmissionService(AdmissionServiceParams{
		Config:                 BillingConfig{Mode: AdmissionModeEnforce, Currency: "CNY"},
		BillingAccountService:  NewBillingAccountService(BillingAccountServiceParams{Ent: client}),
		CommercialLimitService: limitSvc,
	})
	decision, err := admissionSvc.Check(ctx, AdmissionCheckInput{
		Subject:               UserBillingSubject(apiKey.UserID),
		APIKey:                apiKey,
		EstimatedChargeMicros: 1,
	})
	require.ErrorIs(t, err, ErrAPIKeyCommercialLimitExceeded)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeAPIKeyBudgetExceeded, decision.Code)
}

func TestAdmissionRejectsWhenKeyBudgetSufficientButWalletInsufficient(t *testing.T) {
	t.Parallel()

	client, ctx, limitSvc, apiKey, _ := newAPIKeyCommercialLimitTestService(t, "admission_wallet_budget")
	budget := int64(10_000_000)
	apiKey, err := client.APIKey.UpdateOneID(apiKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:           true,
			Currency:          "CNY",
			TotalBudgetMicros: &budget,
		}).
		Save(ctx)
	require.NoError(t, err)

	admissionSvc := NewAdmissionService(AdmissionServiceParams{
		Config:                 BillingConfig{Mode: AdmissionModeEnforce, Currency: "CNY"},
		BillingAccountService:  NewBillingAccountService(BillingAccountServiceParams{Ent: client}),
		CommercialLimitService: limitSvc,
	})
	decision, err := admissionSvc.Check(ctx, AdmissionCheckInput{
		Subject:               UserBillingSubject(apiKey.UserID),
		APIKey:                apiKey,
		EstimatedChargeMicros: 1,
	})
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.False(t, decision.Allowed)
	require.Equal(t, AdmissionCodeInsufficientBalance, decision.Code)
}

func TestUsageBillingProcessorRejectsAPIKeySingleRequestLimit(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_key_single_limit")
	limitSvc := NewAPIKeyCommercialLimitService(APIKeyCommercialLimitServiceParams{
		Ent:           client,
		SystemService: NewSystemService(SystemServiceParams{Ent: client}),
	})
	processor.commercialLimitService = limitSvc
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err := ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)
	_, err = client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Query().Only(ctx)
	require.NoError(t, err)
	singleMax := int64(1_000_000)
	_, err = client.APIKey.UpdateOneID(apiKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:                true,
			Currency:               "CNY",
			SingleRequestMaxMicros: &singleMax,
		}).
		Save(ctx)
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, apiKey.ProjectID, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.True(t, errors.Is(err, ErrAPIKeyCommercialLimitExceeded), "got %v", err)
	require.Nil(t, record)
	count, err := client.UsageBillingRecord.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestUsageBillingProcessorChargedRecordIncrementsAPIKeySpend(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_key_spend")
	limitSvc := NewAPIKeyCommercialLimitService(APIKeyCommercialLimitServiceParams{
		Ent:           client,
		SystemService: NewSystemService(SystemServiceParams{Ent: client}),
	})
	processor.commercialLimitService = limitSvc
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err := ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)
	_, err = client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Query().Only(ctx)
	require.NoError(t, err)
	budget := int64(10_000_000)
	apiKey, err = client.APIKey.UpdateOneID(apiKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:           true,
			Currency:          "CNY",
			TotalBudgetMicros: &budget,
		}).
		Save(ctx)
	require.NoError(t, err)

	before, err := limitSvc.Usage(ctx, apiKey, "CNY", time.Now().UTC())
	require.NoError(t, err)
	require.Zero(t, before.Total.SpentMicros)

	usageLog := createUsageLogForBillingTest(t, client, ctx, apiKey.ProjectID, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)

	after, err := limitSvc.Usage(ctx, apiKey, "CNY", time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, int64(1_500_000), after.Total.SpentMicros)
}

func newAPIKeyCommercialLimitTestService(t *testing.T, name string) (*ent.Client, context.Context, *APIKeyCommercialLimitService, *ent.APIKey, *ent.BillingAccount) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	projectRow, err := client.Project.Create().
		SetName(name).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	password, err := HashPassword("test-password")
	require.NoError(t, err)
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("%s@example.com", name)).
		SetPassword(password).
		SetFirstName("Commercial").
		SetLastName("Limit").
		SetStatus(entuser.StatusActivated).
		Save(ctx)
	require.NoError(t, err)
	apiKeyValue, err := GenerateAPIKey("ah")
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetKey(apiKeyValue).
		SetName("commercial key").
		SetUserID(user.ID).
		SetProjectID(projectRow.ID).
		SetType(apikey.TypeUser).
		Save(ctx)
	require.NoError(t, err)
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	account, err := accountSvc.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	svc := NewAPIKeyCommercialLimitService(APIKeyCommercialLimitServiceParams{
		Ent:           client,
		SystemService: NewSystemService(SystemServiceParams{Ent: client}),
	})

	return client, ctx, svc, apiKey, account
}

func seedCommercialChargedRecord(t *testing.T, client *ent.Client, ctx context.Context, account *ent.BillingAccount, apiKey *ent.APIKey, amountMicros int64, currency string, createdAt time.Time) {
	t.Helper()

	req, err := client.Request.Create().
		SetAPIKeyID(apiKey.ID).
		SetProjectID(apiKey.ProjectID).
		SetModelID("gpt-test").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(apiKey.ID).
		SetProjectID(apiKey.ProjectID).
		SetChannelID(1).
		SetModelID("gpt-test").
		SetPromptTokens(1).
		SetTotalTokens(1).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UsageBillingRecord.Create().
		SetUsageLogID(usageLog.ID).
		SetBillingAccountID(account.ID).
		SetProjectID(apiKey.ProjectID).
		SetUserID(apiKey.UserID).
		SetAPIKeyID(apiKey.ID).
		SetModelID("gpt-test").
		SetPriceSnapshot(testModelPrice("1")).
		SetPriceReferenceID("seed").
		SetCostAmountMicros(0).
		SetChargeAmountMicros(amountMicros).
		SetCurrency(currency).
		SetStatus(usagebillingrecord.StatusCharged).
		SetIdempotencyKey(fmt.Sprintf("seed-commercial:%d", usageLog.ID)).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
}
