package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
	"github.com/looplj/axonhub/internal/objects"
)

func TestUserWorkspaceSummaryScopesCountsToCurrentUserAndProject(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_scoped")
	viewer := createWorkspaceSummaryUser(t, ctx, client, "workspace@example.com")
	other := createWorkspaceSummaryUser(t, ctx, client, "other@example.com")
	selected := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Selected")
	otherProject := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Other")

	selectedKey := createWorkspaceSummaryAPIKey(t, ctx, client, viewer.ID, selected.ID, "Selected Key", apikey.StatusEnabled)
	createWorkspaceSummaryAPIKey(t, ctx, client, viewer.ID, otherProject.ID, "Other Project Key", apikey.StatusEnabled)
	createWorkspaceSummaryAPIKey(t, ctx, client, other.ID, selected.ID, "Other User Key", apikey.StatusEnabled)
	createWorkspaceSummaryRequest(t, ctx, client, selected.ID, selectedKey.ID, request.StatusCompleted)
	createWorkspaceSummaryRequest(t, ctx, client, selected.ID, selectedKey.ID, request.StatusFailed)
	createWorkspaceSummaryChannel(t, ctx, client, "available", []string{"gpt-one", "gpt-two"})
	createWorkspaceSummarySubscription(t, ctx, client, viewer.ID, selected.ID, nil)

	summary, err := svc.GetSummary(ctx, viewer, selected.ID, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, summary.Project)
	require.Equal(t, selected.ID, summary.Project.ID)
	require.Equal(t, 1, summary.APIKeys.Total)
	require.Equal(t, 1, summary.APIKeys.Enabled)
	require.Equal(t, 2, summary.Usage.RequestCount)
	require.Equal(t, 2, summary.Models.AvailableCount)
	require.Equal(t, 1, summary.Subscriptions.ActiveCount)
	require.True(t, summary.Subscriptions.UsableCoverage)
	require.True(t, summary.Onboarding.CanUseAI)
	require.Equal(t, WorkspaceBlockReasonNone, summary.Onboarding.BlockReason)
}

func TestUserWorkspaceSummaryRejectsProjectOutsideMembership(t *testing.T) {
	t.Parallel()

	client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_membership")
	viewer := createWorkspaceSummaryUser(t, ctx, client, "member@example.com")
	other := createWorkspaceSummaryUser(t, ctx, client, "owner@example.com")
	project := createWorkspaceSummaryProject(t, ctx, client, other.ID, "Not Mine")

	_, err := svc.GetSummary(ctx, viewer, project.ID, time.Now().UTC())
	require.ErrorIs(t, err, ErrWorkspaceProjectDenied)
}

func TestUserWorkspaceSummaryOnboardingReasons(t *testing.T) {
	t.Parallel()

	t.Run("missing project", func(t *testing.T) {
		client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_no_project")
		viewer := createWorkspaceSummaryUser(t, ctx, client, "no-project@example.com")

		summary, err := svc.GetSummary(ctx, viewer, 0, time.Now().UTC())
		require.NoError(t, err)
		require.Nil(t, summary.Project)
		require.Equal(t, WorkspaceBlockReasonProjectMissing, summary.Onboarding.BlockReason)
	})

	t.Run("missing api key", func(t *testing.T) {
		client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_no_key")
		viewer := createWorkspaceSummaryUser(t, ctx, client, "no-key@example.com")
		project := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Project")

		summary, err := svc.GetSummary(ctx, viewer, project.ID, time.Now().UTC())
		require.NoError(t, err)
		require.Equal(t, WorkspaceBlockReasonAPIKeyMissing, summary.Onboarding.BlockReason)
	})

	t.Run("missing model", func(t *testing.T) {
		client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_no_model")
		viewer := createWorkspaceSummaryUser(t, ctx, client, "no-model@example.com")
		project := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Project")
		createWorkspaceSummaryAPIKey(t, ctx, client, viewer.ID, project.ID, "Key", apikey.StatusEnabled)

		summary, err := svc.GetSummary(ctx, viewer, project.ID, time.Now().UTC())
		require.NoError(t, err)
		require.Equal(t, WorkspaceBlockReasonModelUnavailable, summary.Onboarding.BlockReason)
	})

	t.Run("insufficient balance", func(t *testing.T) {
		client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_no_balance")
		viewer := createWorkspaceSummaryUser(t, ctx, client, "no-balance@example.com")
		project := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Project")
		createWorkspaceSummaryAPIKey(t, ctx, client, viewer.ID, project.ID, "Key", apikey.StatusEnabled)
		createWorkspaceSummaryChannel(t, ctx, client, "available", []string{"gpt-one"})

		summary, err := svc.GetSummary(ctx, viewer, project.ID, time.Now().UTC())
		require.NoError(t, err)
		require.Equal(t, WorkspaceBlockReasonBalanceInsufficient, summary.Onboarding.BlockReason)
		require.False(t, summary.Onboarding.CanUseAI)
	})

	t.Run("subscription model mismatch", func(t *testing.T) {
		client, ctx, svc := newUserWorkspaceSummaryTestService(t, "workspace_summary_subscription_mismatch")
		viewer := createWorkspaceSummaryUser(t, ctx, client, "subscription-mismatch@example.com")
		project := createWorkspaceSummaryProject(t, ctx, client, viewer.ID, "Project")
		createWorkspaceSummaryAPIKey(t, ctx, client, viewer.ID, project.ID, "Key", apikey.StatusEnabled)
		createWorkspaceSummaryChannel(t, ctx, client, "available", []string{"gpt-one"})
		createWorkspaceSummarySubscription(t, ctx, client, viewer.ID, project.ID, []string{"image-only"})

		summary, err := svc.GetSummary(ctx, viewer, project.ID, time.Now().UTC())
		require.NoError(t, err)
		require.Equal(t, 1, summary.Subscriptions.ActiveCount)
		require.False(t, summary.Subscriptions.UsableCoverage)
		require.Equal(t, WorkspaceBlockReasonBalanceInsufficient, summary.Onboarding.BlockReason)
	})
}

func newUserWorkspaceSummaryTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserWorkspaceSummaryService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	return client, ctx, NewUserWorkspaceSummaryService(UserWorkspaceSummaryServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
	})
}

func createWorkspaceSummaryUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashed-password").
		Save(ctx)
	require.NoError(t, err)
	return user
}

func createWorkspaceSummaryProject(t *testing.T, ctx context.Context, client *ent.Client, userID int, name string) *ent.Project {
	t.Helper()

	projectRow, err := client.Project.Create().
		SetName(name).
		SetStatus(project.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UserProject.Create().
		SetUserID(userID).
		SetProjectID(projectRow.ID).
		SetIsOwner(true).
		Save(ctx)
	require.NoError(t, err)
	return projectRow
}

func createWorkspaceSummaryAPIKey(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, name string, status apikey.Status) *ent.APIKey {
	t.Helper()

	key, err := client.APIKey.Create().
		SetUserID(userID).
		SetProjectID(projectID).
		SetName(name).
		SetKey(name + "-secret").
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)
	return key
}

func createWorkspaceSummaryRequest(t *testing.T, ctx context.Context, client *ent.Client, projectID, apiKeyID int, status request.Status) {
	t.Helper()

	_, err := client.Request.Create().
		SetProjectID(projectID).
		SetAPIKeyID(apiKeyID).
		SetModelID("gpt-one").
		SetRequestBody(objects.JSONRawMessage(`{"model":"gpt-one"}`)).
		SetStatus(status).
		Save(ctx)
	require.NoError(t, err)
}

func createWorkspaceSummaryChannel(t *testing.T, ctx context.Context, client *ent.Client, name string, models []string) {
	t.Helper()

	_, err := client.Channel.Create().
		SetName(name).
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels(models).
		SetDefaultTestModel(models[0]).
		Save(ctx)
	require.NoError(t, err)
}

func createWorkspaceSummarySubscription(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, modelIDs []string) {
	t.Helper()

	now := time.Now().UTC()
	_, err := client.UserSubscription.Create().
		SetUserID(userID).
		SetStatus(usersubscription.StatusActive).
		SetStartsAt(now.Add(-time.Hour)).
		SetExpiresAt(now.Add(time.Hour)).
		SetCurrentPeriodStart(now.Add(-time.Hour)).
		SetCurrentPeriodEnd(now.Add(time.Hour)).
		SetResetAt(now.Add(time.Hour)).
		SetSupportedProjectIds([]int{projectID}).
		SetSupportedModelIds(modelIDs).
		Save(ctx)
	require.NoError(t, err)
}
