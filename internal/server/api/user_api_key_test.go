package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserAPIKeyHandlersRequireAuthentication(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	_, _, handler := newUserAPIKeyHandlerTestService(t, "user_api_key_handler_auth")
	router := gin.New()
	router.GET("/admin/account/api-keys", handler.List)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/api-keys", nil))
	require.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestUserAPIKeyHandlersCreateListAndDenyForeignKey(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserAPIKeyHandlerTestService(t, "user_api_key_handler_lifecycle")
	viewer := createUserAPIKeyHandlerUser(t, ctx, client, "handler@example.com")
	other := createUserAPIKeyHandlerUser(t, ctx, client, "other-handler@example.com")
	workspace := createUserAPIKeyHandlerProject(t, ctx, client, viewer.ID, "Handler")
	addUserAPIKeyHandlerMembership(t, ctx, client, other.ID, workspace.ID)
	foreign := createUserAPIKeyHandlerRow(t, ctx, client, other.ID, workspace.ID, "Foreign", "ah-foreign-handler")

	router := gin.New()
	router.Use(withUserAPIKeyHandlerUser(viewer))
	router.GET("/admin/account/api-keys", handler.List)
	router.POST("/admin/account/api-keys", handler.Create)
	router.PATCH("/admin/account/api-keys", handler.Update)
	router.POST("/admin/account/api-keys/rotate", handler.Rotate)
	router.DELETE("/admin/account/api-keys", handler.Archive)

	projectGUID := "gid://axonhub/Project/" + jsonInt(workspace.ID)
	query := "?projectId=" + url.QueryEscape(projectGUID)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/admin/account/api-keys"+query, bytes.NewBufferString(`{"name":"Browser Key","allowedModelIds":["gpt-safe"],"requestLimit":10,"requestLimitWindow":"minute"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created struct {
		Secret string `json:"secret"`
		APIKey struct {
			ID        string `json:"id"`
			MaskedKey string `json:"maskedKey"`
		} `json:"apiKey"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	require.NotEmpty(t, created.Secret)
	require.NotEqual(t, created.Secret, created.APIKey.MaskedKey)

	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/api-keys"+query, nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.NotContains(t, w.Body.String(), created.Secret)
	require.NotContains(t, w.Body.String(), foreign.Key)

	foreignGUID := "gid://axonhub/APIKey/" + jsonInt(foreign.ID)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/admin/account/api-keys"+query+"&keyId="+url.QueryEscape(foreignGUID), bytes.NewBufferString(`{"name":"Stolen"}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
}

func newUserAPIKeyHandlerTestService(t *testing.T, name string) (*ent.Client, context.Context, *UserAPIKeyHandlers) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := biz.NewProjectService(biz.ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	workspaceSummary := biz.NewUserWorkspaceSummaryService(biz.UserWorkspaceSummaryServiceParams{
		Ent:                   client,
		BillingAccountService: biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client}),
	})
	service := biz.NewUserAPIKeyService(biz.UserAPIKeyServiceParams{
		Ent: client, APIKeyService: apiKeyService, WorkspaceSummaryService: workspaceSummary,
		PricingService: biz.NewPricingService(biz.PricingServiceParams{Ent: client}),
	})
	return client, ctx, NewUserAPIKeyHandlers(UserAPIKeyHandlersParams{Service: service})
}

func createUserAPIKeyHandlerUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()
	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createUserAPIKeyHandlerProject(t *testing.T, ctx context.Context, client *ent.Client, userID int, name string) *ent.Project {
	t.Helper()
	row, err := client.Project.Create().SetName(name).SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	addUserAPIKeyHandlerMembership(t, ctx, client, userID, row.ID)
	return row
}

func addUserAPIKeyHandlerMembership(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int) {
	t.Helper()
	_, err := client.UserProject.Create().SetUserID(userID).SetProjectID(projectID).SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
}

func createUserAPIKeyHandlerRow(t *testing.T, ctx context.Context, client *ent.Client, userID, projectID int, name, key string) *ent.APIKey {
	t.Helper()
	row, err := client.APIKey.Create().SetUserID(userID).SetProjectID(projectID).SetName(name).SetKey(key).SetType(apikey.TypePersonal).Save(ctx)
	require.NoError(t, err)
	return row
}

func withUserAPIKeyHandlerUser(user *ent.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
