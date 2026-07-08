package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	entproject "github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	entuser "github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/objects"
)

func TestBillingReportServiceAggregatesCommercialReport(t *testing.T) {
	t.Parallel()

	client, ctx := newBillingReportTestClient(t, "billing_report_aggregates")
	svc := NewBillingReportService(BillingReportServiceParams{Ent: client})
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	from := now.Add(-48 * time.Hour)
	to := now.Add(24 * time.Hour)

	userA := createBillingReportUser(t, ctx, client, "a@example.com")
	userB := createBillingReportUser(t, ctx, client, "b@example.com")
	projectA := createBillingReportProject(t, ctx, client, "Project A")
	projectB := createBillingReportProject(t, ctx, client, "Project B")
	accountA := createBillingReportAccount(t, ctx, client, userA.ID)
	accountB := createBillingReportAccount(t, ctx, client, userB.ID)

	createBillingReportLedger(t, ctx, client, accountA.ID, ledgertransaction.DirectionCredit, ledgertransaction.TypePaymentRecharge, 10_000_000, now.Add(-26*time.Hour), "recharge-a-1")
	createBillingReportLedger(t, ctx, client, accountA.ID, ledgertransaction.DirectionDebit, ledgertransaction.TypeUsageCharge, 3_000_000, now.Add(-2*time.Hour), "usage-a-1")
	createBillingReportLedger(t, ctx, client, accountB.ID, ledgertransaction.DirectionCredit, ledgertransaction.TypePaymentRecharge, 20_000_000, now.Add(-2*time.Hour), "recharge-b-1")
	createBillingReportLedger(t, ctx, client, accountB.ID, ledgertransaction.DirectionDebit, ledgertransaction.TypeUsageCharge, 4_000_000, now.Add(-1*time.Hour), "usage-b-1")
	createBillingReportLedger(t, ctx, client, accountB.ID, ledgertransaction.DirectionDebit, ledgertransaction.TypeUsageCharge, 2_000_000, now.Add(-50*time.Minute), "usage-b-2")
	createBillingReportLedger(t, ctx, client, accountB.ID, ledgertransaction.DirectionDebit, ledgertransaction.TypeRefund, 1_000_000, now.Add(-30*time.Minute), "refund-b-1")
	createBillingReportLedger(t, ctx, client, accountB.ID, ledgertransaction.DirectionCredit, ledgertransaction.TypePaymentRecharge, 99_000_000, now.Add(-96*time.Hour), "outside-range")

	createBillingReportUsage(t, ctx, client, accountA.ID, userA.ID, projectA.ID, "gpt-4o", 3_000_000, now.Add(-2*time.Hour), usagebillingrecord.StatusCharged, "usage-report-a")
	createBillingReportUsage(t, ctx, client, accountB.ID, userB.ID, projectA.ID, "gpt-4o", 4_000_000, now.Add(-90*time.Minute), usagebillingrecord.StatusCharged, "usage-report-b")
	createBillingReportUsage(t, ctx, client, accountB.ID, userB.ID, projectB.ID, "sora", 2_000_000, now.Add(-80*time.Minute), usagebillingrecord.StatusCharged, "usage-report-c")
	createBillingReportUsage(t, ctx, client, accountA.ID, userA.ID, projectB.ID, "sora", 5_000_000, now.Add(-70*time.Minute), usagebillingrecord.StatusFailed, "usage-report-failed")

	createBillingReportPaymentOrder(t, ctx, client, accountA.ID, projectA.ID, "paid-a", 10_000_000, paymentorder.StatusPaid, now.Add(-26*time.Hour))
	createBillingReportPaymentOrder(t, ctx, client, accountB.ID, projectB.ID, "paid-b", 20_000_000, paymentorder.StatusPaid, now.Add(-2*time.Hour))
	createBillingReportPaymentOrder(t, ctx, client, accountA.ID, projectA.ID, "failed-a", 9_000_000, paymentorder.StatusFailed, now.Add(-1*time.Hour))
	createBillingReportPaymentEvent(t, ctx, client, "failed-event-a", paymentevent.StatusFailed, now.Add(-45*time.Minute))
	createBillingReportHold(t, ctx, client, accountA.ID, userA.ID, projectA.ID, 1_500_000, billinghold.StatusHeld, now.Add(-10*time.Minute))
	createBillingReportHold(t, ctx, client, accountA.ID, userA.ID, projectA.ID, 2_000_000, billinghold.StatusCaptured, now.Add(-5*time.Minute))

	report, err := svc.GetCommercialReport(ctx, BillingReportFilter{From: &from, To: &to, Currency: "CNY", Limit: 5})
	require.NoError(t, err)

	require.Equal(t, int64(30_000_000), report.Summary.RechargeAmountMicros)
	require.Equal(t, int64(9_000_000), report.Summary.ConsumptionAmountMicros)
	require.Equal(t, int64(20_000_000), report.Summary.NetMovementMicros)
	require.Equal(t, int64(1_000_000), report.Summary.RefundAmountMicros)
	require.Equal(t, 1, report.Summary.FailedPaymentCount)
	require.Equal(t, 1, report.Summary.FailedPaymentEventCount)
	require.Equal(t, int64(1_500_000), report.Summary.PendingHoldAmountMicros)
	require.Equal(t, 1, report.Summary.PendingHoldCount)

	require.Len(t, report.Daily, 2)
	require.Equal(t, "2026-07-07", report.Daily[0].Date)
	require.Equal(t, int64(10_000_000), report.Daily[0].RechargeAmountMicros)
	require.Equal(t, "2026-07-08", report.Daily[1].Date)
	require.Equal(t, int64(20_000_000), report.Daily[1].RechargeAmountMicros)

	require.Len(t, report.TopModels, 2)
	require.Equal(t, "gpt-4o", report.TopModels[0].ModelID)
	require.Equal(t, int64(7_000_000), report.TopModels[0].ChargeAmountMicros)
	require.Equal(t, "sora", report.TopModels[1].ModelID)
	require.Equal(t, int64(2_000_000), report.TopModels[1].ChargeAmountMicros)

	require.Len(t, report.TopProjects, 2)
	require.Equal(t, projectA.ID, report.TopProjects[0].ProjectID)
	require.Equal(t, "Project A", report.TopProjects[0].ProjectName)
	require.Equal(t, int64(7_000_000), report.TopProjects[0].ChargeAmountMicros)

	require.Len(t, report.TopUsers, 2)
	require.Equal(t, userB.ID, report.TopUsers[0].UserID)
	require.Equal(t, "b@example.com", report.TopUsers[0].Email)
	require.Equal(t, int64(20_000_000), report.TopUsers[0].RechargeAmountMicros)
	require.Equal(t, int64(6_000_000), report.TopUsers[0].ConsumptionAmountMicros)
}

func TestBillingReportServiceExportsCSV(t *testing.T) {
	t.Parallel()

	client, ctx := newBillingReportTestClient(t, "billing_report_csv")
	svc := NewBillingReportService(BillingReportServiceParams{Ent: client})
	now := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	from := now.Add(-24 * time.Hour)
	to := now.Add(time.Hour)
	user := createBillingReportUser(t, ctx, client, "csv@example.com")
	project := createBillingReportProject(t, ctx, client, "CSV Project")
	account := createBillingReportAccount(t, ctx, client, user.ID)
	createBillingReportLedger(t, ctx, client, account.ID, ledgertransaction.DirectionCredit, ledgertransaction.TypePaymentRecharge, 12_000_000, now, "csv-ledger")
	createBillingReportUsage(t, ctx, client, account.ID, user.ID, project.ID, "gpt-csv", 3_000_000, now, usagebillingrecord.StatusCharged, "csv-usage")
	createBillingReportPaymentOrder(t, ctx, client, account.ID, project.ID, "csv-order", 12_000_000, paymentorder.StatusPaid, now)
	createBillingReportPaymentEvent(t, ctx, client, "csv-event", paymentevent.StatusProcessed, now)

	payload, err := svc.ExportCSV(ctx, BillingCSVExportInput{
		Dataset:  BillingCSVExportDatasetPaymentOrders,
		From:     &from,
		To:       &to,
		Currency: "CNY",
	})
	require.NoError(t, err)
	require.Equal(t, "text/csv", payload.ContentType)
	require.Contains(t, payload.FileName, "payment-orders")
	require.Contains(t, payload.Content, "order_no,project_id,billing_account_id,provider_type,status,amount_micros,currency,external_trade_no,created_at,paid_at")
	require.Contains(t, payload.Content, "csv-order")

	usagePayload, err := svc.ExportCSV(ctx, BillingCSVExportInput{Dataset: BillingCSVExportDatasetUsageBillingRecords, From: &from, To: &to})
	require.NoError(t, err)
	require.Contains(t, usagePayload.Content, "gpt-csv")
	require.True(t, strings.HasSuffix(usagePayload.Content, "\n"))
}

func newBillingReportTestClient(t *testing.T, name string) (*ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	t.Cleanup(func() { client.Close() })
	return client, ctx
}

func createBillingReportUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashed-password").
		SetStatus(entuser.StatusActivated).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func createBillingReportProject(t *testing.T, ctx context.Context, client *ent.Client, name string) *ent.Project {
	t.Helper()

	project, err := client.Project.Create().
		SetName(name).
		SetStatus(entproject.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return project
}

func createBillingReportAccount(t *testing.T, ctx context.Context, client *ent.Client, userID int) *ent.BillingAccount {
	t.Helper()

	account, err := client.BillingAccount.Create().
		SetOwnerType(billingaccount.OwnerTypeUser).
		SetOwnerID(userID).
		SetCurrency("CNY").
		Save(ctx)
	require.NoError(t, err)
	return account
}

func createBillingReportLedger(t *testing.T, ctx context.Context, client *ent.Client, accountID int, direction ledgertransaction.Direction, txType ledgertransaction.Type, amountMicros int64, createdAt time.Time, key string) *ent.LedgerTransaction {
	t.Helper()

	tx, err := client.LedgerTransaction.Create().
		SetCreatedAt(createdAt).
		SetBillingAccountID(accountID).
		SetDirection(direction).
		SetAmountMicros(amountMicros).
		SetCurrency("CNY").
		SetType(txType).
		SetStatus(ledgertransaction.StatusPosted).
		SetIdempotencyKey(key).
		SetReferenceType("test").
		SetReferenceID(key).
		Save(ctx)
	require.NoError(t, err)
	return tx
}

func createBillingReportUsage(t *testing.T, ctx context.Context, client *ent.Client, accountID int, userID int, projectID int, modelID string, chargeMicros int64, createdAt time.Time, status usagebillingrecord.Status, key string) *ent.UsageBillingRecord {
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
		SetModelID(modelID).
		SetPromptTokens(1).
		SetCompletionTokens(1).
		SetTotalTokens(2).
		Save(ctx)
	require.NoError(t, err)
	record, err := client.UsageBillingRecord.Create().
		SetCreatedAt(createdAt).
		SetUsageLogID(usageLog.ID).
		SetBillingAccountID(accountID).
		SetProjectID(projectID).
		SetUserID(userID).
		SetModelID(modelID).
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID("price-" + key).
		SetChargeAmountMicros(chargeMicros).
		SetCurrency("CNY").
		SetStatus(status).
		SetIdempotencyKey(key).
		Save(ctx)
	require.NoError(t, err)
	return record
}

func createBillingReportPaymentOrder(t *testing.T, ctx context.Context, client *ent.Client, accountID int, projectID int, orderNo string, amountMicros int64, status paymentorder.Status, createdAt time.Time) *ent.PaymentOrder {
	t.Helper()

	order, err := client.PaymentOrder.Create().
		SetCreatedAt(createdAt).
		SetOrderNo(orderNo).
		SetProjectID(projectID).
		SetBillingAccountID(accountID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(amountMicros).
		SetCurrency("CNY").
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)
	return order
}

func createBillingReportPaymentEvent(t *testing.T, ctx context.Context, client *ent.Client, eventKey string, status paymentevent.Status, createdAt time.Time) *ent.PaymentEvent {
	t.Helper()

	event, err := client.PaymentEvent.Create().
		SetCreatedAt(createdAt).
		SetEventKey(eventKey).
		SetProviderType(paymentevent.ProviderTypeManual).
		SetEventType("notify").
		SetStatus(status).
		SetPayload(objects.JSONRawMessage([]byte(`{"ok":true}`))).
		Save(ctx)
	require.NoError(t, err)
	return event
}

func createBillingReportHold(t *testing.T, ctx context.Context, client *ent.Client, accountID int, userID int, projectID int, amountMicros int64, status billinghold.Status, createdAt time.Time) *ent.BillingHold {
	t.Helper()

	hold, err := client.BillingHold.Create().
		SetCreatedAt(createdAt).
		SetBillingAccountID(accountID).
		SetUserID(userID).
		SetProjectID(projectID).
		SetModelID("gpt-hold").
		SetAmountMicros(amountMicros).
		SetCurrency("CNY").
		SetStatus(status).
		SetIdempotencyKey("hold-" + string(status) + "-" + createdAt.Format("150405")).
		SetReferenceType("request").
		SetReferenceID(createdAt.Format(time.RFC3339Nano)).
		SetExpiresAt(createdAt.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)
	return hold
}
