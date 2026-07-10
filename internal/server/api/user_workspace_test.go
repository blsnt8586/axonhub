package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserWorkspaceHandlersRequireAuthentication(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	_, _, handler := newUserWorkspaceAPITestService(t, "user_workspace_api_auth")
	router := gin.New()
	router.GET("/admin/account/workspaces", handler.List)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/workspaces", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserWorkspaceHandlersListAndCreate(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserWorkspaceAPITestService(t, "user_workspace_api")
	viewer := createWorkspaceAPIUser(t, ctx, client, "workspace-list@example.com")
	createWorkspaceAPIProject(t, ctx, client, viewer.ID, "Primary")

	router := gin.New()
	router.Use(withUserWorkspaceAPIUser(viewer))
	router.GET("/admin/account/workspaces", handler.List)
	router.POST("/admin/account/workspaces", handler.Create)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/workspaces", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var listed struct {
		Workspaces []struct {
			ID string `json:"id"`
		} `json:"workspaces"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listed))
	require.Len(t, listed.Workspaces, 1)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/account/workspaces", bytes.NewBufferString(`{"name":"Blocked"}`)))
	require.Equal(t, http.StatusForbidden, w.Code)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/account/workspaces", bytes.NewBufferString(`{"name":""}`)))
	require.Equal(t, http.StatusBadRequest, w.Code)

	require.NoError(t, handler.WorkspaceService.SetSettings(ctx, biz.UserWorkspaceSettings{
		AllowSelfServiceCreation: true,
		MaxWorkspacesPerUser:     2,
	}))
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/admin/account/workspaces", bytes.NewBufferString(`{"name":"Primary"}`)))
	require.Equal(t, http.StatusConflict, w.Code)
}

func newUserWorkspaceAPITestService(t *testing.T, name string) (*ent.Client, context.Context, *UserWorkspaceHandlers) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := biz.NewSystemService(biz.SystemServiceParams{Ent: client, CacheConfig: cacheConfig})
	service := biz.NewUserWorkspaceService(biz.UserWorkspaceServiceParams{
		Ent:            client,
		SystemService:  systemService,
		ProjectService: biz.NewProjectService(biz.ProjectServiceParams{Ent: client, CacheConfig: cacheConfig}),
		UserService:    biz.NewUserService(biz.UserServiceParams{Ent: client, CacheConfig: cacheConfig}),
	})
	return client, ctx, NewUserWorkspaceHandlers(UserWorkspaceHandlersParams{WorkspaceService: service})
}

func createWorkspaceAPIUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createWorkspaceAPIProject(t *testing.T, ctx context.Context, client *ent.Client, userID int, name string) {
	t.Helper()
	row, err := client.Project.Create().SetName(name).SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	_, err = client.UserProject.Create().SetUserID(userID).SetProjectID(row.ID).SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
}

func withUserWorkspaceAPIUser(user *ent.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
