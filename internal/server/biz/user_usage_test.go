package biz

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestUserUsageServiceMyRequestsIsolatesUsersAndSensitiveFields(t *testing.T) {
	t.Parallel()
	fixture := newUserUsageFixture(t, "user_usage_isolation")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-viewer@example.com")
	other := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-other@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Usage")
	addUserAPIKeyMembership(t, fixture.ctx, fixture.client, other.ID, workspace.ID)
	viewerKey := createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Viewer Key", "ah-usage-viewer", apikey.TypePersonal)
	otherKey := createUserAPIKeyRow(t, fixture.ctx, fixture.client, other.ID, workspace.ID, "Other Key", "ah-usage-other", apikey.TypePersonal)
	viewerRequest := createUserUsageRequest(t, fixture.ctx, fixture.client, workspace.ID, viewerKey.ID, "gpt-viewer")
	createUserUsageRequest(t, fixture.ctx, fixture.client, workspace.ID, otherKey.ID, "gpt-other")
	createUserUsageBilling(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, viewerKey.ID, viewerRequest.ID, 3200)

	page, err := fixture.service.ListRequests(contexts.WithUser(fixture.ctx, viewer), viewer, UserUsageRequestQuery{
		ProjectID: workspace.ID, Scope: UserUsageScopeMine, Limit: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Len(t, page.Items, 1)
	require.Equal(t, "gpt-viewer", page.Items[0].ModelID)
	require.Equal(t, int64(3200), page.Items[0].ChargeAmountMicros)
	require.Empty(t, page.Items[0].ChannelID)
	require.Empty(t, page.Items[0].UpstreamAccountID)

	detail, err := fixture.service.GetRequest(contexts.WithUser(fixture.ctx, viewer), viewer, workspace.ID, UserUsageScopeMine, viewerRequest.ID)
	require.NoError(t, err)
	require.Equal(t, viewerRequest.ID, mustObjectGUIDID(t, detail.ID, ent.TypeRequest))
	require.NotNil(t, detail.Billing)
	require.NotEmpty(t, detail.Billing.LedgerTransactionID)
	require.NotNil(t, detail.Billing.PriceSnapshot)

	foreignRequest, err := fixture.client.Request.Query().Where(request.ModelIDEQ("gpt-other")).Only(fixture.ctx)
	require.NoError(t, err)
	_, err = fixture.service.GetRequest(contexts.WithUser(fixture.ctx, viewer), viewer, workspace.ID, UserUsageScopeMine, foreignRequest.ID)
	require.ErrorIs(t, err, ErrUserUsageDenied)
}

func TestUserUsageServiceProjectScopeRequiresExplicitRequestReadCapability(t *testing.T) {
	t.Parallel()
	fixture := newUserUsageFixture(t, "user_usage_project_scope")
	owner := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-owner@example.com")
	member := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-member@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, owner.ID, "Project Usage")
	_, err := fixture.client.UserProject.Create().SetUserID(member.ID).SetProjectID(workspace.ID).SetIsOwner(false).Save(fixture.ctx)
	require.NoError(t, err)
	ownerKey := createUserAPIKeyRow(t, fixture.ctx, fixture.client, owner.ID, workspace.ID, "Owner", "ah-owner-usage", apikey.TypePersonal)
	createUserUsageRequest(t, fixture.ctx, fixture.client, workspace.ID, ownerKey.ID, "gpt-owner")

	_, err = fixture.service.ListRequests(contexts.WithUser(fixture.ctx, member), member, UserUsageRequestQuery{
		ProjectID: workspace.ID, Scope: UserUsageScopeProject, Limit: 20,
	})
	require.ErrorIs(t, err, ErrUserUsageProjectAdminRequired)

	ownerWithEdges, err := fixture.client.User.Get(fixture.ctx, owner.ID)
	require.NoError(t, err)
	ownerCtx := contexts.WithProjectID(contexts.WithUser(fixture.ctx, ownerWithEdges), workspace.ID)
	page, err := fixture.service.ListRequests(ownerCtx, ownerWithEdges, UserUsageRequestQuery{
		ProjectID: workspace.ID, Scope: UserUsageScopeProject, Limit: 20,
	})
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
}

func TestUserUsageServiceSummaryReconcilesAggregatesAndBillingRecords(t *testing.T) {
	t.Parallel()
	fixture := newUserUsageFixture(t, "user_usage_summary")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-summary@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Summary")
	key := createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Summary Key", "ah-summary-usage", apikey.TypePersonal)
	req := createUserUsageRequest(t, fixture.ctx, fixture.client, workspace.ID, key.ID, "gpt-summary")
	record := createUserUsageBilling(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, key.ID, req.ID, 4500)
	require.NoError(t, fixture.aggregates.ApplyBillingRecord(fixture.ctx, record))
	other := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-summary-other@example.com")
	addUserAPIKeyMembership(t, fixture.ctx, fixture.client, other.ID, workspace.ID)
	otherKey := createUserAPIKeyRow(t, fixture.ctx, fixture.client, other.ID, workspace.ID, "Other Summary Key", "ah-other-summary-usage", apikey.TypePersonal)
	otherRequest := createUserUsageRequest(t, fixture.ctx, fixture.client, workspace.ID, otherKey.ID, "gpt-summary-other")
	otherRecord := createUserUsageBilling(t, fixture.ctx, fixture.client, other.ID, workspace.ID, otherKey.ID, otherRequest.ID, 9000)
	require.NoError(t, fixture.aggregates.ApplyBillingRecord(fixture.ctx, otherRecord))

	from := time.Now().UTC().Add(-24 * time.Hour)
	to := time.Now().UTC().Add(24 * time.Hour)
	summary, err := fixture.service.Usage(contexts.WithUser(fixture.ctx, viewer), viewer, UserUsageQuery{
		ProjectID: workspace.ID, Scope: UserUsageScopeMine, From: from, To: to,
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), summary.RequestCount)
	require.Equal(t, int64(15), summary.TotalTokens)
	require.Equal(t, int64(4500), summary.ChargeAmountMicros)
	require.Len(t, summary.Series, 1)
	require.Len(t, summary.Models, 1)
	require.Equal(t, "gpt-summary", summary.Models[0].ModelID)
}

func TestUserUsageServiceRejectsCrossProjectListDetailAggregateAndExport(t *testing.T) {
	t.Parallel()
	fixture := newUserUsageFixture(t, "user_usage_cross_project")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-project-viewer@example.com")
	other := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "usage-project-other@example.com")
	viewerWorkspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Viewer Project")
	foreignWorkspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, other.ID, "Foreign Project")
	foreignKey := createUserAPIKeyRow(t, fixture.ctx, fixture.client, other.ID, foreignWorkspace.ID, "Foreign Project Key", "ah-foreign-project-usage", apikey.TypePersonal)
	foreignRequest := createUserUsageRequest(t, fixture.ctx, fixture.client, foreignWorkspace.ID, foreignKey.ID, "gpt-foreign-project")
	viewerCtx := contexts.WithUser(fixture.ctx, viewer)

	_, err := fixture.service.ListRequests(viewerCtx, viewer, UserUsageRequestQuery{ProjectID: foreignWorkspace.ID, Scope: UserUsageScopeMine})
	require.ErrorIs(t, err, ErrUserUsageDenied)
	_, err = fixture.service.GetRequest(viewerCtx, viewer, foreignWorkspace.ID, UserUsageScopeMine, foreignRequest.ID)
	require.ErrorIs(t, err, ErrUserUsageDenied)
	_, err = fixture.service.Usage(viewerCtx, viewer, UserUsageQuery{ProjectID: foreignWorkspace.ID, Scope: UserUsageScopeMine})
	require.ErrorIs(t, err, ErrUserUsageDenied)
	_, err = fixture.service.ExportRequestsCSV(viewerCtx, viewer, UserUsageRequestQuery{ProjectID: foreignWorkspace.ID, Scope: UserUsageScopeMine})
	require.ErrorIs(t, err, ErrUserUsageDenied)

	page, err := fixture.service.ListRequests(viewerCtx, viewer, UserUsageRequestQuery{ProjectID: viewerWorkspace.ID, Scope: UserUsageScopeMine})
	require.NoError(t, err)
	require.Zero(t, page.Total)
}

type userUsageFixture struct {
	client     *ent.Client
	ctx        context.Context
	service    *UserUsageService
	aggregates *UsageAggregateService
}

func newUserUsageFixture(t *testing.T, name string) userUsageFixture {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := NewAPIKeyService(APIKeyServiceParams{Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	userAPIKeys := NewUserAPIKeyService(UserAPIKeyServiceParams{Ent: client, APIKeyService: apiKeyService})
	aggregates := NewUsageAggregateService(UsageAggregateServiceParams{Ent: client})
	return userUsageFixture{
		client: client, ctx: ctx, aggregates: aggregates,
		service: NewUserUsageService(UserUsageServiceParams{Ent: client, UserAPIKeyService: userAPIKeys}),
	}
}

func createUserUsageRequest(t *testing.T, ctx context.Context, client *ent.Client, projectID, apiKeyID int, modelID string) *ent.Request {
	t.Helper()
	row, err := client.Request.Create().
		SetProjectID(projectID).
		SetAPIKeyID(apiKeyID).
		SetSource(request.SourceAPI).
		SetModelID(modelID).
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage(`{"messages":[{"role":"user","content":"hello"}]}`)).
		SetResponseBody(objects.JSONRawMessage(`{"choices":[{"message":{"content":"world"}}]}`)).
		SetRequestHeaders(objects.JSONRawMessage(`{"authorization":"secret"}`)).
		SetClientIP("203.0.113.10").
		Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserUsageBilling(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID, apiKeyID, requestID int, charge int64) *ent.UsageBillingRecord {
	t.Helper()
	usageLog, err := client.UsageLog.Create().
		SetRequestID(requestID).
		SetProjectID(projectID).
		SetAPIKeyID(apiKeyID).
		SetModelID("gpt-summary").
		SetPromptTokens(10).
		SetCompletionTokens(5).
		SetTotalTokens(15).
		Save(ctx)
	require.NoError(t, err)
	account, err := client.BillingAccount.Create().SetOwnerType(billingaccount.OwnerTypeUser).SetOwnerID(userID).SetCurrency("CNY").Save(ctx)
	if err != nil {
		account, err = client.BillingAccount.Query().Only(ctx)
	}
	require.NoError(t, err)
	ledger, err := client.LedgerTransaction.Create().
		SetBillingAccountID(account.ID).
		SetType("usage_charge").
		SetDirection("debit").
		SetAmountMicros(charge).
		SetCurrency("CNY").
		SetStatus("posted").
		SetIdempotencyKey("ledger-" + strconv.Itoa(requestID)).
		Save(ctx)
	require.NoError(t, err)
	record, err := client.UsageBillingRecord.Create().
		SetUsageLogID(usageLog.ID).
		SetBillingAccountID(account.ID).
		SetProjectID(projectID).
		SetUserID(userID).
		SetAPIKeyID(apiKeyID).
		SetModelID(usageLog.ModelID).
		SetPriceSnapshot(testModelPrice("1")).
		SetPriceReferenceID("project-price").
		SetChargeAmountMicros(charge).
		SetCurrency("CNY").
		SetStatus(usagebillingrecord.StatusCharged).
		SetLedgerTransactionID(ledger.ID).
		SetIdempotencyKey("usage-" + strconv.Itoa(requestID)).
		Save(ctx)
	require.NoError(t, err)
	return record
}
