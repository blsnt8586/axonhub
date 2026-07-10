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
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/billingoutbox"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	entuser "github.com/looplj/axonhub/internal/ent/user"
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

func TestPricingServiceSaveUpdateDeleteBillingPriceRule(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:pricing_save_update_delete?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewPricingService(PricingServiceParams{Ent: client})
	enabled := true

	rule, err := svc.SaveBillingPriceRule(ctx, SaveBillingPriceRuleInput{
		ScopeType:    billingpricerule.ScopeTypeProject,
		ScopeID:      42,
		ModelPattern: " gpt-test ",
		Price:        testModelPrice("1"),
		Priority:     10,
		Enabled:      &enabled,
		ReferenceID:  "project-gpt-test-v1",
	})
	require.NoError(t, err)
	require.Equal(t, billingpricerule.ScopeTypeProject, rule.ScopeType)
	require.Equal(t, 42, rule.ScopeID)
	require.Equal(t, "gpt-test", rule.ModelPattern)
	require.Equal(t, "project-gpt-test-v1", rule.ReferenceID)

	disabled := false
	updated, err := svc.SaveBillingPriceRule(ctx, SaveBillingPriceRuleInput{
		ID:           rule.ID,
		ScopeType:    billingpricerule.ScopeTypeProject,
		ScopeID:      42,
		ModelPattern: "gpt-test",
		Price:        testModelPrice("2"),
		Priority:     20,
		Enabled:      &disabled,
		ReferenceID:  "project-gpt-test-v2",
	})
	require.NoError(t, err)
	require.Equal(t, rule.ID, updated.ID)
	require.Equal(t, 20, updated.Priority)
	require.False(t, updated.Enabled)
	require.Equal(t, "project-gpt-test-v2", updated.ReferenceID)

	deleted, err := svc.DeleteBillingPriceRule(ctx, rule.ID)
	require.NoError(t, err)
	require.True(t, deleted)

	deletedAgain, err := svc.DeleteBillingPriceRule(ctx, rule.ID)
	require.NoError(t, err)
	require.False(t, deletedAgain)
}

func TestPricingServiceRejectsUnsupportedBillingPriceRule(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:pricing_reject_invalid?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	svc := NewPricingService(PricingServiceParams{Ent: client})

	_, err := svc.SaveBillingPriceRule(ctx, SaveBillingPriceRuleInput{
		ScopeType:    billingpricerule.ScopeTypeProject,
		ScopeID:      0,
		ModelPattern: "gpt-test",
		Price:        testModelPrice("1"),
	})
	require.Error(t, err)

	_, err = svc.SaveBillingPriceRule(ctx, SaveBillingPriceRuleInput{
		ScopeType:    billingpricerule.ScopeTypeGlobal,
		ScopeID:      0,
		ModelPattern: "",
		Price:        testModelPrice("1"),
	})
	require.Error(t, err)
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

func TestUsageBillingProcessorRoundsLowTokenChargeToMicros(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_low_token_rounding")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-low-token").
		SetPrice(testModelPrice("0.01")).
		SetReferenceID("sell-low-token").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "low-token-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-low-token", 19, 65)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Equal(t, int64(1), record.ChargeAmountMicros)

	transaction, err := client.LedgerTransaction.Get(ctx, record.LedgerTransactionID)
	require.NoError(t, err)
	require.Equal(t, int64(1), transaction.AmountMicros)
}

func TestUsageBillingProcessorStoresRequestTypeFromUsageLogFormat(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_request_type")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("image-test").
		SetPrice(testModelPrice("1")).
		SetReferenceID("image-price").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	apiKey, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(account.OwnerID)).
		First(ctx)
	require.NoError(t, err)
	req, err := client.Request.Create().
		SetAPIKeyID(apiKey.ID).
		SetProjectID(account.OwnerID).
		SetModelID("image-test").
		SetFormat("openai/images").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)
	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(apiKey.ID).
		SetProjectID(account.OwnerID).
		SetChannelID(1).
		SetModelID("image-test").
		SetFormat("openai/images").
		SetPromptTokens(1_000_000).
		SetTotalTokens(1_000_000).
		Save(ctx)
	require.NoError(t, err)

	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.RequestTypeImage, record.RequestType)
}

func TestUsageBillingProcessorCapturesBillingHold(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_capture_hold")
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
	holdSvc := NewBillingHoldService(BillingHoldServiceParams{Ent: client, LedgerService: ledgerSvc})
	processor.billingHoldService = holdSvc
	hold, err := holdSvc.CreateHold(ctx, CreateBillingHoldInput{
		BillingAccountID: account.ID,
		Amount:           decimal.RequireFromString("2"),
		Currency:         "CNY",
		IdempotencyKey:   "usage-hold",
	})
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID, hold.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Equal(t, int64(1_500_000), record.ChargeAmountMicros)
	require.NotZero(t, record.LedgerTransactionID)

	captured, err := client.BillingHold.Get(ctx, hold.ID)
	require.NoError(t, err)
	require.Equal(t, billinghold.StatusCaptured, captured.Status)
	require.Equal(t, int64(1_500_000), captured.CapturedAmountMicros)
	require.Equal(t, record.LedgerTransactionID, captured.CapturedLedgerTransactionID)
	require.Equal(t, usageLog.ID, captured.UsageLogID)

	same, err := processor.BillUsage(ctx, usageLog.ID, hold.ID)
	require.NoError(t, err)
	require.Equal(t, record.ID, same.ID)

	reloaded, err := client.BillingAccount.Get(ctx, account.ID)
	require.NoError(t, err)
	require.Zero(t, reloaded.HeldBalanceMicros)
	require.Equal(t, int64(8_500_000), reloaded.BalanceMicros)
}

func TestUsageBillingProcessorChargesUserWalletWithProjectPrice(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_user_wallet_project_price")
	_, err := client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeProject).
		SetScopeID(1).
		SetModelPattern("gpt-test").
		SetPrice(testModelPrice("2")).
		SetReferenceID("project-sell-v1").
		Save(ctx)
	require.NoError(t, err)
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	_, err = ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("10"), ledgertransaction.TypePaymentRecharge, "initial-credit")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, 1, "gpt-test", 1_000_000, 500_000)
	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.NoError(t, err)
	require.Equal(t, account.ID, record.BillingAccountID)
	require.Equal(t, billingaccount.OwnerTypeUser, account.OwnerType)
	require.Equal(t, account.OwnerID, record.UserID)
	require.Equal(t, 1, record.ProjectID)
	require.Equal(t, "project-sell-v1", record.PriceReferenceID)
	require.Equal(t, int64(3_000_000), record.ChargeAmountMicros)
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

func TestUsageBillingProcessorRejectsRetryOfFailedRecordWithLedgerTransaction(t *testing.T) {
	t.Parallel()

	client, ctx, processor, account := newUsageBillingTestProcessor(t, "usage_billing_failed_with_ledger_retry")
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	ledgerTx, err := ledgerSvc.Credit(ctx, account.ID, decimal.RequireFromString("1"), ledgertransaction.TypePaymentRecharge, "legacy-ledger")
	require.NoError(t, err)

	usageLog := createUsageLogForBillingTest(t, client, ctx, account.OwnerID, "gpt-test", 1_000_000, 0)
	failed, err := client.UsageBillingRecord.Create().
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
		SetLedgerTransactionID(ledgerTx.ID).
		SetIdempotencyKey(usageBillingIdempotencyKey(usageLog.ID)).
		SetError("legacy failed after ledger post").
		Save(ctx)
	require.NoError(t, err)

	record, err := processor.BillUsage(ctx, usageLog.ID)
	require.Error(t, err)
	require.Nil(t, record)
	require.Contains(t, err.Error(), "cannot be retried")

	reloaded, err := client.UsageBillingRecord.Get(ctx, failed.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusFailed, reloaded.Status)
	require.Equal(t, ledgerTx.ID, reloaded.LedgerTransactionID)
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
		SetFirstName("Usage").
		SetLastName("Billing").
		SetStatus(entuser.StatusActivated).
		Save(ctx)
	require.NoError(t, err)
	apiKeyValue, err := GenerateAPIKey("ah")
	require.NoError(t, err)
	_, err = client.APIKey.Create().
		SetKey(apiKeyValue).
		SetName("billing key").
		SetUserID(user.ID).
		SetProjectID(projectRow.ID).
		Save(ctx)
	require.NoError(t, err)
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	account, err := accountSvc.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	pricingSvc := NewPricingService(PricingServiceParams{Ent: client})
	ledgerSvc := NewLedgerService(LedgerServiceParams{Ent: client})
	aggregateSvc := NewUsageAggregateService(UsageAggregateServiceParams{Ent: client})
	processor := NewUsageBillingProcessor(UsageBillingProcessorParams{
		Config:                cfg,
		Ent:                   client,
		PricingService:        pricingSvc,
		BillingAccountService: accountSvc,
		LedgerService:         ledgerSvc,
		UsageAggregateService: aggregateSvc,
	})

	return client, ctx, processor, account
}

func createUsageLogForBillingTest(t *testing.T, client *ent.Client, ctx context.Context, projectID int, modelID string, promptTokens int64, completionTokens int64) *ent.UsageLog {
	t.Helper()

	apiKey, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(projectID)).
		First(ctx)
	require.NoError(t, err)

	req, err := client.Request.Create().
		SetAPIKeyID(apiKey.ID).
		SetProjectID(projectID).
		SetModelID(modelID).
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	usageLog, err := client.UsageLog.Create().
		SetRequestID(req.ID).
		SetAPIKeyID(apiKey.ID).
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
