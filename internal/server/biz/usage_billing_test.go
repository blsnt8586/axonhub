package biz

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingoutbox"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPricingServiceProjectRuleOverridesGlobalRule(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:pricing_project_override?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	global := testModelPrice("1")
	projectPrice := testModelPrice("2")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(global).
		SetReferenceID("global").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeProject).
		SetScopeID(7).
		SetModelPattern("gpt-test").
		SetPrice(projectPrice).
		SetReferenceID("project").
		Save(ctx)
	require.NoError(t, err)

	svc := NewPricingService(PricingServiceParams{Ent: client})
	rule, err := svc.FindSellPrice(ctx, 7, "gpt-test")
	require.NoError(t, err)
	require.Equal(t, "project", rule.ReferenceID)
}

func TestPricingServiceFallsBackToGlobalWildcard(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:pricing_global_wildcard?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("*").
		SetPrice(testModelPrice("1")).
		SetReferenceID("global-wildcard").
		Save(ctx)
	require.NoError(t, err)

	svc := NewPricingService(PricingServiceParams{Ent: client})
	rule, err := svc.FindSellPrice(ctx, 7, "unknown-model")
	require.NoError(t, err)
	require.Equal(t, "global-wildcard", rule.ReferenceID)
}

func TestUsageBillingProcessorChargesUsageOnce(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_charge_once")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Equal(t, int64(1_500_000), record.ChargeAmountMicros)
	require.NotZero(t, record.LedgerTransactionID)

	same, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, same.ID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(8_500_000), reloaded.BalanceMicros)
}

func TestUsageBillingProcessorRequestUsageBillingNoopsWhenBillingDisabled(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessorWithConfig(
		t,
		"usage_billing_request_disabled",
		BillingConfig{Mode: AdmissionModeDisabled},
	)
	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)

	record, err := processor.RequestUsageBilling(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Nil(t, record)

	count, err := client.BillingOutbox.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestUsageBillingProcessorRequestUsageBillingMarksOutboxDone(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_outbox_done")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	record, err := processor.RequestUsageBilling(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)

	outbox, err := client.BillingOutbox.Query().
		Where(billingoutbox.EventKeyEQ(usageBillingIdempotencyKey(usageLog.ID))).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.EventTypeUsageBillingRequested, outbox.EventType)
	require.Equal(t, billingoutbox.StatusDone, outbox.Status)
	require.Equal(t, 1, outbox.Attempts)
	require.Empty(t, outbox.LastError)
	require.Nil(t, outbox.NextAttemptAt)
}

func TestUsageBillingProcessorRequestUsageBillingMarksOutboxFailed(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_outbox_failed")

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "missing-model", 1_000_000, 0)
	record, err := processor.RequestUsageBilling(ctx, usageLog.ID)
	require.ErrorIs(t, err, ErrBillingPriceNotFound)
	require.Nil(t, record)

	outbox, err := client.BillingOutbox.Query().
		Where(billingoutbox.EventKeyEQ(usageBillingIdempotencyKey(usageLog.ID))).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusFailed, outbox.Status)
	require.Equal(t, 1, outbox.Attempts)
	require.Contains(t, outbox.LastError, ErrBillingPriceNotFound.Error())
	require.NotNil(t, outbox.NextAttemptAt)
}

func TestBillingOutboxWorkerSkipsWhenBillingDisabled(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "billing_outbox_worker_disabled")
	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	outbox := createUsageBillingOutboxForTest(t, client, ctx, usageLog.ID, billingoutbox.StatusPending)

	worker := NewBillingOutboxWorker(BillingOutboxWorkerParams{
		Config:                BillingConfig{Mode: AdmissionModeDisabled},
		Ent:                   client,
		UsageBillingProcessor: processor,
	})
	processed, err := worker.ProcessDueOutbox(ctx)
	require.NoError(t, err)
	require.Zero(t, processed)

	reloaded, err := client.BillingOutbox.Get(ctx, outbox.ID)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusPending, reloaded.Status)
	require.Zero(t, reloaded.Attempts)
}

func TestBillingOutboxWorkerProcessesPendingUsageBilling(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "billing_outbox_worker_pending")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	outbox := createUsageBillingOutboxForTest(t, client, ctx, usageLog.ID, billingoutbox.StatusPending)

	worker := NewBillingOutboxWorker(BillingOutboxWorkerParams{
		Config:                BillingConfig{Mode: AdmissionModeWarn},
		Ent:                   client,
		UsageBillingProcessor: processor,
	})
	processed, err := worker.ProcessDueOutbox(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	record, err := client.UsageBillingRecord.Query().
		Where(usagebillingrecord.UsageLogIDEQ(usageLog.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)

	reloadedOutbox, err := client.BillingOutbox.Get(ctx, outbox.ID)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusDone, reloadedOutbox.Status)
	require.Equal(t, 1, reloadedOutbox.Attempts)
	require.Empty(t, reloadedOutbox.LastError)
	require.Nil(t, reloadedOutbox.NextAttemptAt)
}

func TestBillingOutboxWorkerRetriesFailedUsageBillingAfterPriceAndBalanceFixed(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "billing_outbox_worker_retry")
	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)

	record, err := processor.RequestUsageBilling(ctx, usageLog.ID)
	require.ErrorIs(t, err, ErrBillingPriceNotFound)
	require.Nil(t, record)

	outbox, err := client.BillingOutbox.Query().
		Where(billingoutbox.EventKeyEQ(usageBillingIdempotencyKey(usageLog.ID))).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusFailed, outbox.Status)
	require.Equal(t, 1, outbox.Attempts)

	_, err = client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	_, err = client.BillingOutbox.UpdateOneID(outbox.ID).
		SetNextAttemptAt(time.Now().UTC().Add(-time.Minute)).
		Save(ctx)
	require.NoError(t, err)

	worker := NewBillingOutboxWorker(BillingOutboxWorkerParams{
		Config:                BillingConfig{Mode: AdmissionModeWarn},
		Ent:                   client,
		UsageBillingProcessor: processor,
	})
	processed, err := worker.ProcessDueOutbox(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	charged, err := client.UsageBillingRecord.Query().
		Where(usagebillingrecord.UsageLogIDEQ(usageLog.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, charged.Status)
	require.Equal(t, int64(1_000_000), charged.ChargeAmountMicros)

	reloadedOutbox, err := client.BillingOutbox.Get(ctx, outbox.ID)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusDone, reloadedOutbox.Status)
	require.Equal(t, 2, reloadedOutbox.Attempts)
	require.Empty(t, reloadedOutbox.LastError)
	require.Nil(t, reloadedOutbox.NextAttemptAt)
}

func TestBillingOutboxWorkerRetriesLegacyFailedUsageBillingRecord(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "billing_outbox_worker_legacy_failed")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	legacyFailed, err := client.UsageBillingRecord.Create().
		SetUsageLogID(usageLog.ID).
		SetBillingAccountID(account.ID).
		SetProjectID(usageLog.ProjectID).
		SetModelID(usageLog.ModelID).
		SetUsageSnapshot(objects.JSONRawMessage([]byte(`{}`))).
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("").
		SetChargeAmountMicros(0).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusFailed).
		SetIdempotencyKey(usageBillingIdempotencyKey(usageLog.ID)).
		SetError("legacy missing price").
		Save(ctx)
	require.NoError(t, err)
	outbox := createUsageBillingOutboxForTest(t, client, ctx, usageLog.ID, billingoutbox.StatusPending)

	worker := NewBillingOutboxWorker(BillingOutboxWorkerParams{
		Config:                BillingConfig{Mode: AdmissionModeWarn},
		Ent:                   client,
		UsageBillingProcessor: processor,
	})
	processed, err := worker.ProcessDueOutbox(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	_, err = client.UsageBillingRecord.Get(ctx, legacyFailed.ID)
	require.True(t, ent.IsNotFound(err))

	charged, err := client.UsageBillingRecord.Query().
		Where(usagebillingrecord.UsageLogIDEQ(usageLog.ID)).
		Only(ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, charged.Status)
	require.NotEqual(t, legacyFailed.ID, charged.ID)

	reloadedOutbox, err := client.BillingOutbox.Get(ctx, outbox.ID)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusDone, reloadedOutbox.Status)
}

func TestUsageBillingProcessorDoesNotCreateRecordWhenPriceMissing(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_missing_price")
	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "missing-model", 1_000_000, 0)

	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.ErrorIs(t, err, ErrBillingPriceNotFound)
	require.Nil(t, record)

	count, err := client.UsageBillingRecord.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestUsageBillingProcessorDoesNotCreateRecordWhenBalanceInsufficient(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_insufficient_balance")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("sell-v1").
		Save(ctx)
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Nil(t, record)

	count, err := client.UsageBillingRecord.Query().Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func newUsageBillingTestProcessor(t *testing.T, name string) (*ent.Client, context.Context, *UsageBillingProcessor, *ent.BillingAccount) {
	t.Helper()

	return newUsageBillingTestProcessorWithConfig(t, name, BillingConfig{Mode: AdmissionModeWarn})
}

func newUsageBillingTestProcessorWithConfig(t *testing.T, name string, cfg BillingConfig) (*ent.Client, context.Context, *UsageBillingProcessor, *ent.BillingAccount) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	_, err := client.Project.Create().
		SetName(name).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	account, err := accountSvc.GetOrCreateForSubject(ctx, ProjectBillingSubject(1))
	require.NoError(t, err)
	pricingSvc := NewPricingService(PricingServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	processor := NewUsageBillingProcessor(UsageBillingProcessorParams{
		Config:                cfg,
		Ent:                   client,
		PricingService:        pricingSvc,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
	})

	return client, ctx, processor, account
}

func createUsageLogForBillingTest(t *testing.T, client *ent.Client, ctx context.Context, projectID int, modelID string, promptTokens int64, completionTokens int64) *ent.UsageLog {
	t.Helper()

	req, err := client.Request.Create().
		SetProjectID(projectID).
		SetModelID(modelID).
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetProjectID(projectID).
		SetChannelID(1).
		SetModelID(modelID).
		SetPromptTokens(promptTokens).
		SetCompletionTokens(completionTokens).
		SetTotalTokens(promptTokens + completionTokens).
		SetTotalCost(0.25).
		Save(ctx)
	require.NoError(t, err)

	return usageLog
}

func createUsageBillingOutboxForTest(t *testing.T, client *ent.Client, ctx context.Context, usageLogID int, status billingoutbox.Status) *ent.BillingOutbox {
	t.Helper()

	outbox, err := client.BillingOutbox.Create().
		SetEventKey(usageBillingIdempotencyKey(usageLogID)).
		SetEventType(billingoutbox.EventTypeUsageBillingRequested).
		SetPayload(objects.JSONRawMessage([]byte(fmt.Sprintf(`{"usage_log_id":%d}`, usageLogID)))).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)

	return outbox
}

func testModelPrice(unitPrice string) objects.ModelPrice {
	price := decimal.RequireFromString(unitPrice)
	return objects.ModelPrice{
		Items: []objects.ModelPriceItem{
			{
				ItemCode: objects.PriceItemCodeUsage,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: &price,
				},
			},
			{
				ItemCode: objects.PriceItemCodeCompletion,
				Pricing: objects.Pricing{
					Mode:         objects.PricingModeUsagePerUnit,
					UsagePerUnit: &price,
				},
			},
		},
	}
}
