package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/role"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/scopes"
)

const SystemKeyUserWorkspaceSettings = "workspace_settings"

const defaultMaxWorkspacesPerUser = 1

type UserWorkspaceSettings struct {
	AllowSelfServiceCreation bool `json:"allow_self_service_creation"`
	MaxWorkspacesPerUser     int  `json:"max_workspaces_per_user"`
}

type UserWorkspaceCapabilities struct {
	ConsumeAI        bool `json:"consumeAI"`
	ManageOwnAPIKeys bool `json:"manageOwnAPIKeys"`
	ViewOwnUsage     bool `json:"viewOwnUsage"`
	ManageMembers    bool `json:"manageMembers"`
	ManageRoles      bool `json:"manageRoles"`
	ManageSharedKeys bool `json:"manageSharedKeys"`
}

type UserWorkspace struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Description  string                    `json:"description"`
	Status       project.Status            `json:"status"`
	IsOwner      bool                      `json:"isOwner"`
	Capabilities UserWorkspaceCapabilities `json:"capabilities"`
}

type UserWorkspaceList struct {
	Settings           UserWorkspacePublicSettings `json:"settings"`
	DefaultWorkspaceID string                      `json:"defaultWorkspaceId,omitempty"`
	Workspaces         []UserWorkspace             `json:"workspaces"`
}

type UserWorkspacePublicSettings struct {
	AllowSelfServiceCreation bool `json:"allowSelfServiceCreation"`
	MaxWorkspacesPerUser     int  `json:"maxWorkspacesPerUser"`
}

type CreateUserWorkspaceInput struct {
	Name        string
	Description string
}

type UserWorkspaceServiceParams struct {
	fx.In

	Ent            *ent.Client
	SystemService  *SystemService
	ProjectService *ProjectService
	UserService    *UserService
}

type UserWorkspaceService struct {
	*AbstractService

	SystemService  *SystemService
	ProjectService *ProjectService
	UserService    *UserService
}

func NewUserWorkspaceService(params UserWorkspaceServiceParams) *UserWorkspaceService {
	return &UserWorkspaceService{
		AbstractService: &AbstractService{db: params.Ent},
		SystemService:   params.SystemService,
		ProjectService:  params.ProjectService,
		UserService:     params.UserService,
	}
}

func DefaultUserWorkspaceSettings() UserWorkspaceSettings {
	return UserWorkspaceSettings{MaxWorkspacesPerUser: defaultMaxWorkspacesPerUser}
}

func (s *UserWorkspaceService) Settings(ctx context.Context) (*UserWorkspaceSettings, error) {
	value, err := s.SystemService.getSystemValue(ctx, SystemKeyUserWorkspaceSettings)
	if err != nil {
		if ent.IsNotFound(err) {
			settings := DefaultUserWorkspaceSettings()
			return &settings, nil
		}
		return nil, fmt.Errorf("failed to get workspace settings: %w", err)
	}

	settings := DefaultUserWorkspaceSettings()
	if err := json.Unmarshal([]byte(value), &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal workspace settings: %w", err)
	}
	if settings.MaxWorkspacesPerUser <= 0 {
		settings.MaxWorkspacesPerUser = defaultMaxWorkspacesPerUser
	}
	return &settings, nil
}

func (s *UserWorkspaceService) SetSettings(ctx context.Context, settings UserWorkspaceSettings) error {
	if settings.MaxWorkspacesPerUser <= 0 {
		return fmt.Errorf("max workspaces per user must be greater than zero")
	}

	value, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("failed to marshal workspace settings: %w", err)
	}
	if err := s.SystemService.setSystemValue(ctx, SystemKeyUserWorkspaceSettings, string(value)); err != nil {
		return fmt.Errorf("failed to set workspace settings: %w", err)
	}
	return nil
}

func (s *UserWorkspaceService) List(ctx context.Context, viewer *ent.User) (*UserWorkspaceList, error) {
	if viewer == nil {
		return nil, ErrWorkspaceProjectDenied
	}

	return authz.RunWithSystemBypass(ctx, "user-workspace-list", func(bypassCtx context.Context) (*UserWorkspaceList, error) {
		settings, err := s.Settings(bypassCtx)
		if err != nil {
			return nil, err
		}

		client := s.entFromContext(bypassCtx)
		memberships, err := client.UserProject.Query().
			Where(
				userproject.UserIDEQ(viewer.ID),
				userproject.HasProjectWith(project.StatusEQ(project.StatusActive)),
			).
			WithProject().
			Order(userproject.ByProjectID()).
			All(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to list user workspaces: %w", err)
		}

		projectIDs := make([]int, 0, len(memberships))
		for _, membership := range memberships {
			projectIDs = append(projectIDs, membership.ProjectID)
		}

		roleScopes := make(map[int][]string, len(projectIDs))
		if len(projectIDs) > 0 {
			roles, err := client.Role.Query().Where(
				role.ProjectIDIn(projectIDs...),
				role.HasUsersWith(user.IDEQ(viewer.ID)),
			).All(bypassCtx)
			if err != nil {
				return nil, fmt.Errorf("failed to list user workspace roles: %w", err)
			}
			for _, assignedRole := range roles {
				if assignedRole.ProjectID != nil {
					roleScopes[*assignedRole.ProjectID] = append(roleScopes[*assignedRole.ProjectID], assignedRole.Scopes...)
				}
			}
		}

		result := &UserWorkspaceList{
			Settings: UserWorkspacePublicSettings{
				AllowSelfServiceCreation: settings.AllowSelfServiceCreation,
				MaxWorkspacesPerUser:     settings.MaxWorkspacesPerUser,
			},
			Workspaces: make([]UserWorkspace, 0, len(memberships)),
		}
		for _, membership := range memberships {
			projectRow, err := membership.Edges.ProjectOrErr()
			if err != nil {
				return nil, fmt.Errorf("failed to load user workspace: %w", err)
			}
			workspace := projectWorkspace(viewer, projectRow, membership, roleScopes[projectRow.ID])
			result.Workspaces = append(result.Workspaces, workspace)
			if result.DefaultWorkspaceID == "" && membership.IsOwner {
				result.DefaultWorkspaceID = workspace.ID
			}
		}
		if result.DefaultWorkspaceID == "" && len(result.Workspaces) > 0 {
			result.DefaultWorkspaceID = result.Workspaces[0].ID
		}
		return result, nil
	})
}

func (s *UserWorkspaceService) Create(ctx context.Context, viewer *ent.User, input CreateUserWorkspaceInput) (*UserWorkspace, error) {
	if viewer == nil {
		return nil, ErrWorkspaceProjectDenied
	}
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil || currentUser.ID != viewer.ID {
		return nil, ErrWorkspaceProjectDenied
	}

	name := strings.TrimSpace(input.Name)
	if utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 80 {
		return nil, ErrWorkspaceInvalidName
	}
	description := strings.TrimSpace(input.Description)

	bypassCtx := authz.WithSystemBypass(ctx, "user-workspace-create")
	settings, err := s.Settings(bypassCtx)
	if err != nil {
		return nil, err
	}
	if !settings.AllowSelfServiceCreation {
		return nil, ErrWorkspaceCreationDisabled
	}

	var created *ent.Project
	err = s.RunInTransaction(bypassCtx, func(txCtx context.Context) error {
		client := s.entFromContext(txCtx)
		count, err := client.UserProject.Query().Where(
			userproject.UserIDEQ(viewer.ID),
			userproject.HasProjectWith(project.StatusEQ(project.StatusActive)),
		).Count(txCtx)
		if err != nil {
			return fmt.Errorf("failed to count user workspaces: %w", err)
		}
		if count >= settings.MaxWorkspacesPerUser {
			return ErrWorkspaceLimitReached
		}

		createInput := ent.CreateProjectInput{Name: name}
		if description != "" {
			createInput.Description = &description
		}
		created, err = s.ProjectService.CreateProject(contexts.WithUser(txCtx, viewer), createInput)
		return err
	})
	if err != nil {
		return nil, err
	}
	s.UserService.invalidateUserCache(bypassCtx, viewer.ID)

	membership := &ent.UserProject{UserID: viewer.ID, ProjectID: created.ID, IsOwner: true}
	workspace := projectWorkspace(viewer, created, membership, nil)
	return &workspace, nil
}

func projectWorkspace(viewer *ent.User, projectRow *ent.Project, membership *ent.UserProject, assignedRoleScopes []string) UserWorkspace {
	scopeSet := make(map[string]struct{}, len(membership.Scopes)+len(assignedRoleScopes))
	for _, scope := range membership.Scopes {
		scopeSet[scope] = struct{}{}
	}
	for _, scope := range assignedRoleScopes {
		scopeSet[scope] = struct{}{}
	}

	canManage := func(scope scopes.ScopeSlug) bool {
		if viewer.IsOwner || membership.IsOwner {
			return true
		}
		_, ok := scopeSet[string(scope)]
		return ok
	}
	active := viewer.Status == user.StatusActivated && projectRow.Status == project.StatusActive

	return UserWorkspace{
		ID:          workspaceGUID(projectRow.ID),
		Name:        projectRow.Name,
		Description: projectRow.Description,
		Status:      projectRow.Status,
		IsOwner:     viewer.IsOwner || membership.IsOwner,
		Capabilities: UserWorkspaceCapabilities{
			ConsumeAI:        active,
			ManageOwnAPIKeys: active,
			ViewOwnUsage:     true,
			ManageMembers:    canManage(scopes.ScopeWriteUsers),
			ManageRoles:      canManage(scopes.ScopeWriteRoles),
			ManageSharedKeys: canManage(scopes.ScopeWriteAPIKeys),
		},
	}
}

func workspaceGUID(projectID int) string {
	return fmt.Sprintf("gid://axonhub/%s/%d", ent.TypeProject, projectID)
}
