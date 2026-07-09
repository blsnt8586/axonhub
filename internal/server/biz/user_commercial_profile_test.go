package biz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	entproject "github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
)

func TestUserCommercialProfileServiceUserCannotReadAnotherUser(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newCommercialProfileTestService(t, "commercial_profile_access_denied")
	userA := createCommercialProfileUser(t, ctx, client, "a@example.com", false)
	userB := createCommercialProfileUser(t, ctx, client, "b@example.com", false)

	_, err := svc.GetProfile(ctx, userA, userB.ID, UserCommercialProfileFilter{})
	require.ErrorIs(t, err, ErrCommercialProfileDenied)
}

func TestUserCommercialProfileServiceOwnerCanInspectUserAndReconcileTotals(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newCommercialProfileTestService(t, "commercial_profile_owner_inspect")
	owner := createCommercialProfileUser(t, ctx, client, "owner@example.com", true)
	user := createCommercialProfileUser(t, ctx, client, "user@example.com", false)
	projectA := createCommercialProfileProject(t, ctx, client, "Project A")
	projectB := createCommercialProfileProject(t, ctx, client, "Project B")
	apiKeyA := createCommercialProfileAPIKey(t, ctx, client, user.ID, projectA.ID, "Primary Key")
	apiKeyB := createCommercialProfileAPIKey(t, ctx, client, user.ID, projectB.ID, "Secondary Key")
	account, err := svc.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)
	_, err = client.BillingAccount.UpdateOneID(account.ID).
		SetBalanceMicros(30_000_000).
		SetHeldBalanceMicros(2_000_000).
		SetCreditLimitMicros(5_000_000).
		Save(ctx)
	require.NoError(t, err)

	now := time.Now().UTC()
	createCommercialProfileLedger(t, ctx, client, account.ID, "recharge-1", ledgertransaction.DirectionCredit, ledgertransaction.TypePaymentRecharge, 20_000_000, now.Add(-2*time.Hour))
	createCommercialProfileLedger(t, ctx, client, account.ID, "recharge-2", ledgertransaction.DirectionCredit, ledgertransaction.TypeAdminAdjustment, 5_000_000, now.Add(-time.Hour))
	createCommercialProfileLedger(t, ctx, client, account.ID, "debit-ignored", ledgertransaction.DirectionDebit, ledgertransaction.TypeAdminAdjustment, 1_000_000, now.Add(-time.Hour))

	usageA := createCommercialProfileUsageLog(t, ctx, client, projectA.ID, apiKeyA.ID, "gpt-4o", request.StatusCompleted, "openai/chat_completions", now.Add(-40*time.Minute))
	usageB := createCommercialProfileUsageLog(t, ctx, client, projectA.ID, apiKeyA.ID, "gpt-4o", request.StatusCompleted, "openai/chat_completions", now.Add(-30*time.Minute))
	usageC := createCommercialProfileUsageLog(t, ctx, client, projectB.ID, apiKeyB.ID, "imagen", request.StatusCompleted, "openai/images", now.Add(-20*time.Minute))
	usageFailed := createCommercialProfileUsageLog(t, ctx, client, projectB.ID, apiKeyB.ID, "imagen", request.StatusFailed, "openai/images", now.Add(-10*time.Minute))

	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageA.ID, projectA.ID, apiKeyA.ID, "gpt-4o", usagebillingrecord.RequestTypeChat, usagebillingrecord.StatusCharged, 3_000_000, "", now.Add(-39*time.Minute))
	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageB.ID, projectA.ID, apiKeyA.ID, "gpt-4o", usagebillingrecord.RequestTypeChat, usagebillingrecord.StatusCharged, 4_000_000, "", now.Add(-29*time.Minute))
	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageC.ID, projectB.ID, apiKeyB.ID, "imagen", usagebillingrecord.RequestTypeImage, usagebillingrecord.StatusCharged, 2_000_000, "", now.Add(-19*time.Minute))
	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageFailed.ID, projectB.ID, apiKeyB.ID, "imagen", usagebillingrecord.RequestTypeImage, usagebillingrecord.StatusFailed, 1_000_000, "upstream rejected", now.Add(-9*time.Minute))

	profile, err := svc.GetProfile(ctx, owner, user.ID, UserCommercialProfileFilter{Limit: 5})
	require.NoError(t, err)
	require.Equal(t, user.ID, profile.User.ID)
	require.Equal(t, int64(30_000_000), profile.BillingAccount.BalanceMicros)
	require.Equal(t, int64(33_000_000), profile.BillingAccount.AvailableMicros)
	require.Equal(t, int64(25_000_000), profile.Totals.TotalRechargeMicros)
	require.Equal(t, int64(9_000_000), profile.Totals.TotalConsumptionMicros)
	require.Equal(t, 4, profile.Totals.RequestCount)
	require.Equal(t, 2, profile.Totals.FailureCount)
	require.Len(t, profile.TopModels, 2)
	require.Equal(t, "gpt-4o", profile.TopModels[0].ID)
	require.Equal(t, int64(7_000_000), profile.TopModels[0].ChargeAmountMicros)
	require.Len(t, profile.TopProjects, 2)
	require.Equal(t, "Project A", profile.TopProjects[0].Name)
	require.Len(t, profile.TopAPIKeys, 2)
	require.Equal(t, "Primary Key", profile.TopAPIKeys[0].Name)
	require.Len(t, profile.RecentCharges, 4)
	require.Len(t, profile.RecentRequests, 4)
	require.Len(t, profile.RecentBillingFailures, 1)
	require.Equal(t, "upstream rejected", profile.RecentBillingFailures[0].Error)
}

func TestUserCommercialProfileServiceFiltersUsageAndRequests(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newCommercialProfileTestService(t, "commercial_profile_filters")
	user := createCommercialProfileUser(t, ctx, client, "filter@example.com", false)
	projectA := createCommercialProfileProject(t, ctx, client, "Project A")
	projectB := createCommercialProfileProject(t, ctx, client, "Project B")
	apiKeyA := createCommercialProfileAPIKey(t, ctx, client, user.ID, projectA.ID, "Chat Key")
	apiKeyB := createCommercialProfileAPIKey(t, ctx, client, user.ID, projectB.ID, "Image Key")
	account, err := svc.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(user.ID))
	require.NoError(t, err)

	now := time.Now().UTC()
	usageA := createCommercialProfileUsageLog(t, ctx, client, projectA.ID, apiKeyA.ID, "gpt-filter", request.StatusCompleted, "openai/chat_completions", now.Add(-2*time.Hour))
	usageB := createCommercialProfileUsageLog(t, ctx, client, projectB.ID, apiKeyB.ID, "image-filter", request.StatusCompleted, "openai/images", now.Add(-time.Hour))
	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageA.ID, projectA.ID, apiKeyA.ID, "gpt-filter", usagebillingrecord.RequestTypeChat, usagebillingrecord.StatusCharged, 2_000_000, "", now.Add(-2*time.Hour))
	createCommercialProfileUsageRecord(t, ctx, client, account.ID, user.ID, usageB.ID, projectB.ID, apiKeyB.ID, "image-filter", usagebillingrecord.RequestTypeImage, usagebillingrecord.StatusCharged, 6_000_000, "", now.Add(-time.Hour))

	profile, err := svc.GetProfile(ctx, user, user.ID, UserCommercialProfileFilter{
		ProjectID:   &projectB.ID,
		APIKeyID:    &apiKeyB.ID,
		RequestType: string(usagebillingrecord.RequestTypeImage),
		Limit:       5,
	})
	require.NoError(t, err)
	require.Equal(t, int64(6_000_000), profile.Totals.TotalConsumptionMicros)
	require.Equal(t, 1, profile.Totals.RequestCount)
	require.Len(t, profile.TopModels, 1)
	require.Equal(t, "image-filter", profile.TopModels[0].ID)
	require.Len(t, profile.RecentCharges, 1)
	require.Equal(t, projectB.ID, profile.RecentCharges[0].ProjectID)
	require.Len(t, profile.RecentRequests, 1)
	require.Equal(t, "image", profile.RecentRequests[0].RequestType)
}

func newCommercialProfileTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserCommercialProfileService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	svc := NewUserCommercialProfileService(UserCommercialProfileServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
	})

	return client, ctx, svc
}

func createCommercialProfileUser(t *testing.T, ctx context.Context, client *ent.Client, email string, isOwner bool) *ent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashed-password").
		SetIsOwner(isOwner).
		Save(ctx)
	require.NoError(t, err)

	return user
}

func createCommercialProfileProject(t *testing.T, ctx context.Context, client *ent.Client, name string) *ent.Project {
	t.Helper()

	project, err := client.Project.Create().
		SetName(name).
		SetStatus(entproject.StatusActive).
		Save(ctx)
	require.NoError(t, err)

	return project
}

func createCommercialProfileAPIKey(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, name string) *ent.APIKey {
	t.Helper()

	key, err := client.APIKey.Create().
		SetUserID(userID).
		SetProjectID(projectID).
		SetName(name).
		SetKey(name + "-secret").
		Save(ctx)
	require.NoError(t, err)

	return key
}

func createCommercialProfileLedger(t *testing.T, ctx context.Context, client *ent.Client, accountID int, key string, direction ledgertransaction.Direction, txType ledgertransaction.Type, amountMicros int64, createdAt time.Time) *ent.LedgerTransaction {
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
		Save(ctx)
	require.NoError(t, err)

	return tx
}

func createCommercialProfileUsageLog(t *testing.T, ctx context.Context, client *ent.Client, projectID, apiKeyID int, modelID string, status request.Status, format string, createdAt time.Time) *ent.UsageLog {
	t.Helper()

	req, err := client.Request.Create().
		SetCreatedAt(createdAt).
		SetAPIKeyID(apiKeyID).
		SetProjectID(projectID).
		SetModelID(modelID).
		SetFormat(format).
		SetStatus(status).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(ctx)
	require.NoError(t, err)

	usageLog, err := client.UsageLog.Create().
		SetCreatedAt(createdAt).
		SetRequestID(req.ID).
		SetAPIKeyID(apiKeyID).
		SetProjectID(projectID).
		SetChannelID(1).
		SetModelID(modelID).
		SetTotalTokens(1000).
		SetTotalCost(0.01).
		Save(ctx)
	require.NoError(t, err)

	return usageLog
}

func createCommercialProfileUsageRecord(t *testing.T, ctx context.Context, client *ent.Client, accountID, userID, usageLogID, projectID, apiKeyID int, modelID string, requestType usagebillingrecord.RequestType, status usagebillingrecord.Status, chargeMicros int64, errText string, createdAt time.Time) *ent.UsageBillingRecord {
	t.Helper()

	record, err := client.UsageBillingRecord.Create().
		SetCreatedAt(createdAt).
		SetUsageLogID(usageLogID).
		SetBillingAccountID(accountID).
		SetProjectID(projectID).
		SetUserID(userID).
		SetAPIKeyID(apiKeyID).
		SetModelID(modelID).
		SetRequestType(requestType).
		SetPriceSnapshot(objects.ModelPrice{}).
		SetPriceReferenceID(modelID + "-price").
		SetChargeAmountMicros(chargeMicros).
		SetCurrency("CNY").
		SetStatus(status).
		SetError(errText).
		SetIdempotencyKey(modelID + "-" + status.String() + "-" + time.Now().Format(time.RFC3339Nano)).
		Save(ctx)
	require.NoError(t, err)

	return record
}

func TestUserCommercialProfileServiceInvalidRange(t *testing.T) {
	t.Parallel()

	_, ctx, svc := newCommercialProfileTestService(t, "commercial_profile_invalid_range")
	user := &ent.User{ID: 1}
	from := time.Now().UTC()
	to := from.Add(-time.Minute)

	_, err := svc.GetProfile(ctx, user, user.ID, UserCommercialProfileFilter{From: &from, To: &to})
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrCommercialProfileDenied))
}
