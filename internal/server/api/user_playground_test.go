package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserPlaygroundHandlersReturnNoModelInsteadOfAccessDenied(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserPlaygroundHandlerFixture(t, "user_playground_handler_state")
	viewer := createUserAPIKeyHandlerUser(t, ctx, client, "playground-handler@example.com")
	workspace := createUserAPIKeyHandlerProject(t, ctx, client, viewer.ID, "Playground Handler")

	router := gin.New()
	router.Use(withUserAPIKeyHandlerUser(viewer))
	router.GET("/admin/account/playground", handler.State)
	projectGUID := "gid://axonhub/Project/" + jsonInt(workspace.ID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/account/playground?projectId="+url.QueryEscape(projectGUID), nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"blockReason":"no_model"`)
	require.NotContains(t, w.Body.String(), "channel")
}

func TestUserPlaygroundChatInjectsOwnedAPIKeyAndRejectsForeignKey(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	client, ctx, handler := newUserPlaygroundHandlerFixture(t, "user_playground_handler_chat")
	viewer := createUserAPIKeyHandlerUser(t, ctx, client, "chat-handler@example.com")
	other := createUserAPIKeyHandlerUser(t, ctx, client, "chat-handler-other@example.com")
	workspace := createUserAPIKeyHandlerProject(t, ctx, client, viewer.ID, "Chat Handler")
	addUserAPIKeyHandlerMembership(t, ctx, client, other.ID, workspace.ID)
	owned := createUserAPIKeyHandlerRow(t, ctx, client, viewer.ID, workspace.ID, "Owned", "ah-handler-owned")
	foreign := createUserAPIKeyHandlerRow(t, ctx, client, other.ID, workspace.ID, "Foreign", "ah-handler-foreign")
	_, err := client.Channel.Create().
		SetName("Chat upstream").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-handler"}).
		SetDefaultTestModel("gpt-handler").
		Save(ctx)
	require.NoError(t, err)

	called := false
	handler.ChatCompletion = func(c *gin.Context) {
		called = true
		key, ok := contexts.GetAPIKey(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, owned.ID, key.ID)
		projectID, ok := contexts.GetProjectID(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, workspace.ID, projectID)
		principal, ok := authz.GetPrincipal(c.Request.Context())
		require.True(t, ok)
		require.Equal(t, authz.PrincipalTypeAPIKey, principal.Type)
		require.Empty(t, c.GetHeader("X-Channel-ID"))
		require.Empty(t, c.GetHeader("X-Project-ID"))
		require.NotContains(t, c.Request.URL.Query(), "channel_id")
		require.NotContains(t, c.Request.URL.Query(), "project_id")
		require.True(t, c.GetBool(consumerPlaygroundContextKey))
		c.Status(http.StatusNoContent)
	}

	router := gin.New()
	router.Use(withUserAPIKeyHandlerUser(viewer))
	router.POST("/admin/account/playground/chat", handler.Chat)
	projectGUID := "gid://axonhub/Project/" + jsonInt(workspace.ID)
	base := "/admin/account/playground/chat?projectId=" + url.QueryEscape(projectGUID) + "&keyId="

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, base+url.QueryEscape("gid://axonhub/APIKey/"+jsonInt(foreign.ID)), bytes.NewBufferString(`{"model":"gpt-handler","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.False(t, called)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, base+url.QueryEscape("gid://axonhub/APIKey/"+jsonInt(owned.ID)), bytes.NewBufferString(`{"model":"gpt-handler","messages":[]}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Channel-ID", "gid://axonhub/Channel/999")
	req.Header.Set("X-Project-ID", "gid://axonhub/Project/999")
	query := req.URL.Query()
	query.Set("channel_id", "gid://axonhub/Channel/999")
	query.Set("project_id", "gid://axonhub/Project/999")
	req.URL.RawQuery = query.Encode()
	req.RemoteAddr = "203.0.113.10:1234"
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	require.True(t, called)
}

func newUserPlaygroundHandlerFixture(t *testing.T, name string) (*ent.Client, context.Context, *UserPlaygroundHandlers) {
	t.Helper()
	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := biz.NewProjectService(biz.ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := biz.NewAPIKeyService(biz.APIKeyServiceParams{Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	billingAccountService := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	workspaceSummary := biz.NewUserWorkspaceSummaryService(biz.UserWorkspaceSummaryServiceParams{Ent: client, BillingAccountService: billingAccountService})
	pricing := biz.NewPricingService(biz.PricingServiceParams{Ent: client})
	userAPIKeys := biz.NewUserAPIKeyService(biz.UserAPIKeyServiceParams{
		Ent: client, APIKeyService: apiKeyService, WorkspaceSummaryService: workspaceSummary, PricingService: pricing,
	})
	service := biz.NewUserPlaygroundService(biz.UserPlaygroundServiceParams{
		Ent: client, UserAPIKeyService: userAPIKeys, WorkspaceSummaryService: workspaceSummary, PricingService: pricing,
		AdmissionService: biz.NewAdmissionService(biz.AdmissionServiceParams{
			Config: biz.BillingConfig{Mode: biz.AdmissionModeDisabled, Currency: "CNY"}, BillingAccountService: billingAccountService,
		}),
	})
	return client, ctx, &UserPlaygroundHandlers{Service: service}
}
