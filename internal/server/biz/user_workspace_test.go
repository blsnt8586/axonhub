package biz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/scopes"
)

func TestUserWorkspaceServiceListsOnlyActiveMembershipsAndProjectsCapabilities(t *testing.T) {
	t.Parallel()

	client, ctx, service := newUserWorkspaceTestService(t, "user_workspace_list")
	viewer := createUserWorkspaceUser(t, ctx, client, "viewer@example.com")
	other := createUserWorkspaceUser(t, ctx, client, "other@example.com")
	owned := createUserWorkspaceProject(t, ctx, client, viewer.ID, "Owned", project.StatusActive, true, nil)
	member := createUserWorkspaceProject(t, ctx, client, viewer.ID, "Member", project.StatusActive, false, []string{
		string(scopes.ScopeReadUsers),
	})
	createUserWorkspaceProject(t, ctx, client, viewer.ID, "Archived", project.StatusArchived, true, nil)
	createUserWorkspaceProject(t, ctx, client, other.ID, "Foreign", project.StatusActive, true, nil)

	developer, err := client.Role.Create().
		SetName("Workspace Developer").
		SetLevel(role.LevelProject).
		SetProjectID(member.ID).
		SetScopes([]string{string(scopes.ScopeWriteAPIKeys), string(scopes.ScopeWriteRoles)}).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UserRole.Create().SetUserID(viewer.ID).SetRoleID(developer.ID).Save(ctx)
	require.NoError(t, err)

	result, err := service.List(ctx, viewer)
	require.NoError(t, err)
	require.Equal(t, workspaceGUID(owned.ID), result.DefaultWorkspaceID)
	require.Len(t, result.Workspaces, 2)

	require.Equal(t, workspaceGUID(owned.ID), result.Workspaces[0].ID)
	require.True(t, result.Workspaces[0].IsOwner)
	require.True(t, result.Workspaces[0].Capabilities.ConsumeAI)
	require.True(t, result.Workspaces[0].Capabilities.ManageOwnAPIKeys)
	require.True(t, result.Workspaces[0].Capabilities.ViewOwnUsage)
	require.True(t, result.Workspaces[0].Capabilities.ManageMembers)
	require.True(t, result.Workspaces[0].Capabilities.ManageRoles)
	require.True(t, result.Workspaces[0].Capabilities.ManageSharedKeys)

	require.Equal(t, workspaceGUID(member.ID), result.Workspaces[1].ID)
	require.False(t, result.Workspaces[1].IsOwner)
	require.True(t, result.Workspaces[1].Capabilities.ConsumeAI)
	require.True(t, result.Workspaces[1].Capabilities.ManageOwnAPIKeys)
	require.True(t, result.Workspaces[1].Capabilities.ViewOwnUsage)
	require.False(t, result.Workspaces[1].Capabilities.ManageMembers)
	require.True(t, result.Workspaces[1].Capabilities.ManageRoles)
	require.True(t, result.Workspaces[1].Capabilities.ManageSharedKeys)
}

func TestUserWorkspaceServiceCreateHonorsCommercialPolicy(t *testing.T) {
	t.Parallel()

	t.Run("disabled", func(t *testing.T) {
		client, ctx, service := newUserWorkspaceTestService(t, "user_workspace_disabled")
		viewer := createUserWorkspaceUser(t, ctx, client, "disabled@example.com")

		_, err := service.Create(contexts.WithUser(ctx, viewer), viewer, CreateUserWorkspaceInput{Name: "Personal"})
		require.ErrorIs(t, err, ErrWorkspaceCreationDisabled)
	})

	t.Run("limit reached", func(t *testing.T) {
		client, ctx, service := newUserWorkspaceTestService(t, "user_workspace_limit")
		viewer := createUserWorkspaceUser(t, ctx, client, "limit@example.com")
		createUserWorkspaceProject(t, ctx, client, viewer.ID, "Existing", project.StatusActive, true, nil)
		require.NoError(t, service.SetSettings(ctx, UserWorkspaceSettings{
			AllowSelfServiceCreation: true,
			MaxWorkspacesPerUser:     1,
		}))

		_, err := service.Create(contexts.WithUser(ctx, viewer), viewer, CreateUserWorkspaceInput{Name: "Second"})
		require.ErrorIs(t, err, ErrWorkspaceLimitReached)
	})

	t.Run("creates complete owner workspace", func(t *testing.T) {
		client, ctx, service := newUserWorkspaceTestService(t, "user_workspace_create")
		viewer := createUserWorkspaceUser(t, ctx, client, "create@example.com")
		_, err := service.UserService.GetUserByID(ctx, viewer.ID)
		require.NoError(t, err)
		require.NoError(t, service.SetSettings(ctx, UserWorkspaceSettings{
			AllowSelfServiceCreation: true,
			MaxWorkspacesPerUser:     2,
		}))

		created, err := service.Create(contexts.WithUser(ctx, viewer), viewer, CreateUserWorkspaceInput{Name: "  Personal Lab  "})
		require.NoError(t, err)
		require.Equal(t, "Personal Lab", created.Name)
		require.True(t, created.IsOwner)
		require.True(t, created.Capabilities.ManageMembers)

		projectID := mustWorkspaceID(t, created.ID)
		membership, err := client.UserProject.Query().Where(
			userproject.UserIDEQ(viewer.ID), userproject.ProjectIDEQ(projectID),
		).Only(ctx)
		require.NoError(t, err)
		require.True(t, membership.IsOwner)

		roles, err := client.Role.Query().Where(role.ProjectIDEQ(projectID)).All(ctx)
		require.NoError(t, err)
		require.Len(t, roles, 3)

		refreshedUser, err := service.UserService.GetUserByID(ctx, viewer.ID)
		require.NoError(t, err)
		require.Len(t, refreshedUser.Edges.ProjectUsers, 1)
	})
}

func mustWorkspaceID(t *testing.T, guid string) int {
	t.Helper()
	parsed, err := objects.ParseGUID(guid)
	require.NoError(t, err)
	require.Equal(t, ent.TypeProject, parsed.Type)
	return parsed.ID
}

func TestUserWorkspaceSettingsNormalizeAndValidate(t *testing.T) {
	t.Parallel()

	_, ctx, service := newUserWorkspaceTestService(t, "user_workspace_settings")
	settings, err := service.Settings(ctx)
	require.NoError(t, err)
	require.False(t, settings.AllowSelfServiceCreation)
	require.Equal(t, 1, settings.MaxWorkspacesPerUser)

	require.Error(t, service.SetSettings(ctx, UserWorkspaceSettings{
		AllowSelfServiceCreation: true,
		MaxWorkspacesPerUser:     0,
	}))
}

func newUserWorkspaceTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserWorkspaceService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: cacheConfig})
	projectService := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	userService := NewUserService(UserServiceParams{Ent: client, CacheConfig: cacheConfig})
	return client, ctx, NewUserWorkspaceService(UserWorkspaceServiceParams{
		Ent:            client,
		SystemService:  systemService,
		ProjectService: projectService,
		UserService:    userService,
	})
}

func createUserWorkspaceUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserWorkspaceProject(
	t *testing.T,
	ctx context.Context,
	client *ent.Client,
	userID int,
	name string,
	status project.Status,
	isOwner bool,
	directScopes []string,
) *ent.Project {
	t.Helper()
	row, err := client.Project.Create().SetName(name).SetStatus(status).Save(ctx)
	require.NoError(t, err)
	_, err = client.UserProject.Create().
		SetUserID(userID).
		SetProjectID(row.ID).
		SetIsOwner(isOwner).
		SetScopes(directScopes).
		Save(ctx)
	require.NoError(t, err)
	return row
}
