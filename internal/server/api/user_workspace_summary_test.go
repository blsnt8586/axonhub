package api

import (
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
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserWorkspaceSummaryHandlerReturnsCurrentUserWorkspace(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, handler := newWorkspaceSummaryAPITestService(t, "api_workspace_summary")
	viewer := createWorkspaceSummaryAPIUser(t, ctx, client, "workspace-api@example.com")
	projectRow := createWorkspaceSummaryAPIProject(t, ctx, client, viewer.ID, "Workspace")

	router := gin.New()
	router.Use(withWorkspaceSummaryAPIUser(viewer))
	router.GET("/admin/account/workspace-summary", handler.GetMySummary)
	req := httptest.NewRequest(http.MethodGet, "/admin/account/workspace-summary?projectId="+url.QueryEscape("gid://axonhub/Project/"+jsonInt(projectRow.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		User struct {
			ID int `json:"id"`
		} `json:"user"`
		Project struct {
			ID int `json:"id"`
		} `json:"project"`
		Onboarding struct {
			BlockReason string `json:"blockReason"`
		} `json:"onboarding"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, viewer.ID, body.User.ID)
	require.Equal(t, projectRow.ID, body.Project.ID)
	require.Equal(t, string(biz.WorkspaceBlockReasonAPIKeyMissing), body.Onboarding.BlockReason)
}

func TestUserWorkspaceSummaryHandlerRejectsForeignProject(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, handler := newWorkspaceSummaryAPITestService(t, "api_workspace_summary_denied")
	viewer := createWorkspaceSummaryAPIUser(t, ctx, client, "viewer-api@example.com")
	other := createWorkspaceSummaryAPIUser(t, ctx, client, "other-api@example.com")
	projectRow := createWorkspaceSummaryAPIProject(t, ctx, client, other.ID, "Foreign")

	router := gin.New()
	router.Use(withWorkspaceSummaryAPIUser(viewer))
	router.GET("/admin/account/workspace-summary", handler.GetMySummary)
	req := httptest.NewRequest(http.MethodGet, "/admin/account/workspace-summary?projectId="+url.QueryEscape("gid://axonhub/Project/"+jsonInt(projectRow.ID)), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func newWorkspaceSummaryAPITestService(t *testing.T, name string) (*ent.Client, context.Context, *UserWorkspaceSummaryHandlers) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	service := biz.NewUserWorkspaceSummaryService(biz.UserWorkspaceSummaryServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
	})
	return client, ctx, NewUserWorkspaceSummaryHandlers(UserWorkspaceSummaryHandlersParams{SummaryService: service})
}

func createWorkspaceSummaryAPIUser(t *testing.T, ctx context.Context, client *ent.Client, email string) *ent.User {
	t.Helper()

	row, err := client.User.Create().SetEmail(email).SetPassword("hashed-password").Save(ctx)
	require.NoError(t, err)
	return row
}

func createWorkspaceSummaryAPIProject(t *testing.T, ctx context.Context, client *ent.Client, userID int, name string) *ent.Project {
	t.Helper()

	row, err := client.Project.Create().SetName(name).SetStatus(project.StatusActive).Save(ctx)
	require.NoError(t, err)
	_, err = client.UserProject.Create().SetUserID(userID).SetProjectID(row.ID).SetIsOwner(true).Save(ctx)
	require.NoError(t, err)
	return row
}

func withWorkspaceSummaryAPIUser(user *ent.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), user))
		c.Next()
	}
}
