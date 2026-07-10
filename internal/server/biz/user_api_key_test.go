package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestUserAPIKeyServiceListsOnlyCurrentUsersKeysAndMasksSecrets(t *testing.T) {
	t.Parallel()

	client, ctx, service := newUserAPIKeyTestService(t, "user_api_key_list")
	viewer := createUserAPIKeyUser(t, ctx, client, "viewer@example.com")
	other := createUserAPIKeyUser(t, ctx, client, "other@example.com")
	workspace := createUserAPIKeyProject(t, ctx, client, viewer.ID, "Shared")
	addUserAPIKeyMembership(t, ctx, client, other.ID, workspace.ID)
	owned := createUserAPIKeyRow(t, ctx, client, viewer.ID, workspace.ID, "Owned", "ah-owned-secret", apikey.TypePersonal)
	createUserAPIKeyRow(t, ctx, client, other.ID, workspace.ID, "Foreign", "ah-foreign-secret", apikey.TypePersonal)

	keys, err := service.List(contexts.WithUser(ctx, viewer), viewer, workspace.ID)
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, workspaceObjectGUID(owned.ID, ent.TypeAPIKey), keys[0].ID)
	require.NotEqual(t, owned.Key, keys[0].MaskedKey)
	require.NotContains(t, keys[0].MaskedKey, "owned-secret")
}

func TestUserAPIKeyServiceLifecyclePreservesRuntimeProfiles(t *testing.T) {
	t.Parallel()

	client, ctx, service := newUserAPIKeyTestService(t, "user_api_key_lifecycle")
	viewer := createUserAPIKeyUser(t, ctx, client, "lifecycle@example.com")
	workspace := createUserAPIKeyProject(t, ctx, client, viewer.ID, "Lifecycle")
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	dailyBudget := int64(5_000_000)
	requestLimit := int64(20)

	created, err := service.Create(contexts.WithUser(ctx, viewer), viewer, UserAPIKeyCreateInput{
		ProjectID:          workspace.ID,
		Name:               "CLI Key",
		ExpiresAt:          &expiresAt,
		IPAllowlist:        []string{"203.0.113.10", "10.0.0.0/8"},
		AllowedModelIDs:    []string{"gpt-test"},
		RequestLimit:       &requestLimit,
		RequestLimitWindow: UserAPIKeyLimitWindowMinute,
		CommercialLimits: &objects.APIKeyCommercialLimits{
			Enabled:           true,
			Currency:          "CNY",
			DailyBudgetMicros: &dailyBudget,
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.Secret)
	require.Equal(t, apikey.TypePersonal, created.APIKey.Type)
	require.Equal(t, []string{"gpt-test"}, created.APIKey.AllowedModelIDs)
	require.Equal(t, &requestLimit, created.APIKey.RequestLimit)
	require.Equal(t, UserAPIKeyLimitWindowMinute, created.APIKey.RequestLimitWindow)

	rowID := mustObjectGUIDID(t, created.APIKey.ID, ent.TypeAPIKey)
	stored, err := client.APIKey.Get(ctx, rowID)
	require.NoError(t, err)
	require.Equal(t, created.Secret, stored.Key)
	require.Equal(t, []string{"203.0.113.10/32", "10.0.0.0/8"}, stored.IPAllowlist)
	require.Equal(t, "Personal", stored.Profiles.ActiveProfile)
	require.Equal(t, []string{"gpt-test"}, stored.Profiles.Profiles[0].ModelIDs)
	require.Equal(t, &requestLimit, stored.Profiles.Profiles[0].Quota.Requests)

	newName := "Updated CLI Key"
	newModels := []string{"gpt-test", "claude-test"}
	updated, err := service.Update(contexts.WithUser(ctx, viewer), viewer, workspace.ID, rowID, UserAPIKeyUpdateInput{
		Name:            &newName,
		AllowedModelIDs: &newModels,
		ClearExpiresAt:  true,
	})
	require.NoError(t, err)
	require.Equal(t, newName, updated.Name)
	require.Nil(t, updated.ExpiresAt)
	require.Equal(t, newModels, updated.AllowedModelIDs)

	rotated, err := service.Rotate(contexts.WithUser(ctx, viewer), viewer, workspace.ID, rowID)
	require.NoError(t, err)
	require.NotEmpty(t, rotated.Secret)
	require.NotEqual(t, created.Secret, rotated.Secret)
	require.NotEqual(t, rotated.Secret, rotated.APIKey.MaskedKey)

	require.NoError(t, service.Archive(contexts.WithUser(ctx, viewer), viewer, workspace.ID, rowID))
	listed, err := service.List(contexts.WithUser(ctx, viewer), viewer, workspace.ID)
	require.NoError(t, err)
	require.Empty(t, listed)
}

func TestUserAPIKeyServiceRejectsForeignWorkspaceAndForeignKey(t *testing.T) {
	t.Parallel()

	client, ctx, service := newUserAPIKeyTestService(t, "user_api_key_denied")
	viewer := createUserAPIKeyUser(t, ctx, client, "denied@example.com")
	other := createUserAPIKeyUser(t, ctx, client, "owner@example.com")
	foreignWorkspace := createUserAPIKeyProject(t, ctx, client, other.ID, "Foreign")
	sharedWorkspace := createUserAPIKeyProject(t, ctx, client, viewer.ID, "Shared")
	addUserAPIKeyMembership(t, ctx, client, other.ID, sharedWorkspace.ID)
	foreignKey := createUserAPIKeyRow(t, ctx, client, other.ID, sharedWorkspace.ID, "Other Key", "ah-other-secret", apikey.TypePersonal)

	_, err := service.Create(contexts.WithUser(ctx, viewer), viewer, UserAPIKeyCreateInput{
		ProjectID: foreignWorkspace.ID,
		Name:      "Denied",
	})
	require.ErrorIs(t, err, ErrUserAPIKeyDenied)

	_, err = service.Update(contexts.WithUser(ctx, viewer), viewer, sharedWorkspace.ID, foreignKey.ID, UserAPIKeyUpdateInput{})
	require.ErrorIs(t, err, ErrUserAPIKeyDenied)
	_, err = service.Rotate(contexts.WithUser(ctx, viewer), viewer, sharedWorkspace.ID, foreignKey.ID)
	require.ErrorIs(t, err, ErrUserAPIKeyDenied)
	require.ErrorIs(t, service.Archive(contexts.WithUser(ctx, viewer), viewer, sharedWorkspace.ID, foreignKey.ID), ErrUserAPIKeyDenied)
}

func TestValidateAPIKeyRequestAccessEnforcesExpiryAndIPAllowlist(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	expired := now.Add(-time.Minute)
	key := &ent.APIKey{ExpiresAt: &expired}
	require.ErrorIs(t, ValidateAPIKeyRequestAccess(key, "203.0.113.10", now), ErrInvalidAPIKey)

	future := now.Add(time.Hour)
	key.ExpiresAt = &future
	key.IPAllowlist = []string{"203.0.113.0/24", "2001:db8::/32"}
	require.NoError(t, ValidateAPIKeyRequestAccess(key, "203.0.113.10", now))
	require.NoError(t, ValidateAPIKeyRequestAccess(key, "2001:db8::10", now))
	require.ErrorIs(t, ValidateAPIKeyRequestAccess(key, "198.51.100.10", now), ErrInvalidAPIKey)
	require.ErrorIs(t, ValidateAPIKeyRequestAccess(key, "", now), ErrInvalidAPIKey)
}

func TestUserAPIKeyServiceModelsReturnsConsumerSafeCatalog(t *testing.T) {
	t.Parallel()

	client, ctx, service := newUserAPIKeyTestService(t, "user_api_key_models")
	viewer := createUserAPIKeyUser(t, ctx, client, "models@example.com")
	workspace := createUserAPIKeyProject(t, ctx, client, viewer.ID, "Models")
	_, err := client.Channel.Create().
		SetName("Private Channel").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-safe"}).
		SetDefaultTestModel("gpt-safe").
		Save(ctx)
	require.NoError(t, err)

	models, err := service.Models(contexts.WithUser(ctx, viewer), viewer, workspace.ID)
	require.NoError(t, err)
	require.Equal(t, []UserAPIKeyModel{{ModelID: "gpt-safe"}}, models)
}

func newUserAPIKeyTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserAPIKeyService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah",
	})
	t.Cleanup(apiKeyService.Stop)
	workspaceSummaryService := NewUserWorkspaceSummaryService(UserWorkspaceSummaryServiceParams{
		Ent:                   client,
		BillingAccountService: NewBillingAccountService(BillingAccountServiceParams{Ent: client}),
	})
	return client, ctx, NewUserAPIKeyService(UserAPIKeyServiceParams{
		Ent: client, APIKeyService: apiKeyService,
		WorkspaceSummaryService: workspaceSummaryService,
		PricingService:          NewPricingService(PricingServiceParams{Ent: client}),
	})
}

func createUserAPIKeyUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserAPIKeyProject(t *testing.T, ctx context.Context, client *ent.Client, userID int, name string) *ent.Project {
	t.Helper()
	row, err := client.Project.Create().SetName(name).SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	addUserAPIKeyMembership(t, ctx, client, userID, row.ID)
	return row
}

func addUserAPIKeyMembership(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int) {
	t.Helper()
	_, err := client.UserProject.Create().SetUserID(userID).SetProjectID(projectID).SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
}

func createUserAPIKeyRow(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	userID, projectID int,
	name, secret string,
	keyType apikey.Type,
) *ent.APIKey {
	t.Helper()
	row, err := client.APIKey.Create().
		SetUserID(userID).
		SetProjectID(projectID).
		SetName(name).
		SetKey(secret).
		SetType(keyType).
		Save(ctx)
	require.NoError(t, err)
	return row
}

func mustObjectGUIDID(t *testing.T, raw, expectedType string) int {
	t.Helper()
	parsed, err := objects.ParseGUID(raw)
	require.NoError(t, err)
	require.Equal(t, expectedType, parsed.Type)
	return parsed.ID
}
