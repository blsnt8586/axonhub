package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserUsageHandlersRequireAuthentication(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	_, _, handler := newUserUsageHandlerTestService(t, "user_usage_handler_auth")
	router := gin.New()
	router.GET("/admin/account/requests", handler.MyRequests)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/requests", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserUsageHandlersIsolateMineAndProtectProjectScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserUsageHandlerTestService(t, "user_usage_handler_isolation")
	owner := createUserUsageHandlerUser(t, ctx, client, "usage-owner@example.com")
	viewer := createUserUsageHandlerUser(t, ctx, client, "usage-viewer@example.com")
	workspace := createUserUsageHandlerProject(t, ctx, client, owner.ID, "Usage API")
	addUserUsageHandlerMembership(t, ctx, client, viewer.ID, workspace.ID, false)
	ownerKey := createUserUsageHandlerKey(t, ctx, client, owner.ID, workspace.ID, "Owner Key", "ah-owner-secret")
	viewerKey := createUserUsageHandlerKey(t, ctx, client, viewer.ID, workspace.ID, "Viewer Key", "ah-viewer-secret")
	ownerRequest := createUserUsageHandlerRequest(t, ctx, client, workspace.ID, ownerKey.ID, "owner-model", "owner-private-marker")
	viewerRequest := createUserUsageHandlerRequest(t, ctx, client, workspace.ID, viewerKey.ID, "viewer-model", "viewer-private-marker")

	router := gin.New()
	router.UseRawPath = true
	router.UnescapePathValues = true
	router.Use(withUserUsageHandlerUser(viewer))
	router.GET("/admin/account/requests", handler.MyRequests)
	router.GET("/admin/account/requests/export", handler.ExportMyRequests)
	router.GET("/admin/account/requests/:request_id", handler.MyRequest)
	router.GET("/admin/account/project-requests", handler.ProjectRequests)

	projectID := "gid://axonhub/Project/" + strconv.Itoa(workspace.ID)
	query := "?projectId=" + url.QueryEscape(projectID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/requests"+query, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "viewer-model")
	require.NotContains(t, w.Body.String(), "owner-model")
	require.NotContains(t, w.Body.String(), "authorization")
	require.NotContains(t, w.Body.String(), "ah-viewer-secret")

	foreignID := "gid://axonhub/Request/" + strconv.Itoa(ownerRequest.ID)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/requests/"+url.PathEscape(foreignID)+query, nil))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	mineID := "gid://axonhub/Request/" + strconv.Itoa(viewerRequest.ID)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/requests/"+url.PathEscape(mineID)+query, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "viewer-private-marker")
	require.NotContains(t, w.Body.String(), "authorization")
	require.NotContains(t, w.Body.String(), "clientIp")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/requests/export"+query, nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, "text/csv; charset=utf-8", w.Header().Get("Content-Type"))
	require.Contains(t, w.Body.String(), "viewer-model")
	require.NotContains(t, w.Body.String(), "owner-model")

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/project-requests"+query, nil))
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
}

func TestUserUsageHandlersAllowProjectOwnerScope(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserUsageHandlerTestService(t, "user_usage_handler_owner")
	owner := createUserUsageHandlerUser(t, ctx, client, "project-owner@example.com")
	member := createUserUsageHandlerUser(t, ctx, client, "project-member@example.com")
	workspace := createUserUsageHandlerProject(t, ctx, client, owner.ID, "Owner Usage")
	addUserUsageHandlerMembership(t, ctx, client, member.ID, workspace.ID, false)
	memberKey := createUserUsageHandlerKey(t, ctx, client, member.ID, workspace.ID, "Member Key", "ah-member-secret")
	createUserUsageHandlerRequest(t, ctx, client, workspace.ID, memberKey.ID, "member-model", "member-body")

	router := gin.New()
	router.UseRawPath = true
	router.UnescapePathValues = true
	router.Use(withUserUsageHandlerUser(owner))
	router.GET("/admin/account/project-requests", handler.ProjectRequests)
	projectID := "gid://axonhub/Project/" + strconv.Itoa(workspace.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/project-requests?projectId="+url.QueryEscape(projectID), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "member-model")
}

func newUserUsageHandlerTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserUsageHandlers) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := biz.NewProjectService(biz.ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	userAPIKeys := biz.NewUserAPIKeyService(biz.UserAPIKeyServiceParams{Ent: client, APIKeyService: apiKeyService})
	service := biz.NewUserUsageService(biz.UserUsageServiceParams{Ent: client, UserAPIKeyService: userAPIKeys})
	return client, ctx, NewUserUsageHandlers(UserUsageHandlersParams{Service: service})
}

func createUserUsageHandlerUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserUsageHandlerProject(t *testing.T, ctx context.Context, client *ent.Client, ownerID int, name string) *ent.Project {
	t.Helper()
	row, err := client.Project.Create().SetName(name).SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	addUserUsageHandlerMembership(t, ctx, client, ownerID, row.ID, true)
	return row
}

func addUserUsageHandlerMembership(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, owner bool) {
	t.Helper()
	_, err := client.UserProject.Create().SetUserID(userID).SetProjectID(projectID).SetIsOwner(owner).Save(ctx)
	require.NoError(t, err)
}

func createUserUsageHandlerKey(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, name, key string) *ent.APIKey {
	t.Helper()
	row, err := client.APIKey.Create().SetUserID(userID).SetProjectID(projectID).SetName(name).SetKey(key).SetType(apikey.TypePersonal).Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserUsageHandlerRequest(t *testing.T, ctx context.Context, client *ent.Client, projectID, keyID int, modelID, marker string) *ent.Request {
	t.Helper()
	body, err := json.Marshal(map[string]any{"model": modelID, "marker": marker})
	require.NoError(t, err)
	row, err := client.Request.Create().
		SetProjectID(projectID).
		SetAPIKeyID(keyID).
		SetSource(request.SourceAPI).
		SetStatus(request.StatusCompleted).
		SetModelID(modelID).
		SetRequestHeaders(objects.JSONRawMessage(`{"authorization":"sensitive"}`)).
		SetClientIP("203.0.113.25").
		SetRequestBody(objects.JSONRawMessage(body)).
		SetResponseBody(objects.JSONRawMessage(`{"ok":true}`)).
		Save(ctx)
	require.NoError(t, err)
	return row
}

func withUserUsageHandlerUser(user *ent.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := contexts.WithUser(c.Request.Context(), user)
		ctx = authz.NewUserContext(ctx, user.ID)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
