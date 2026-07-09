package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
)

func TestUserCommercialProfileHandlersRejectCrossUserRead(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, handlers := newCommercialProfileAPITestService(t, "api_commercial_profile_cross_user")
	viewer := createCommercialProfileAPIUser(t, ctx, client, "viewer@example.com", false)
	other := createCommercialProfileAPIUser(t, ctx, client, "other@example.com", false)

	router := gin.New()
	router.Use(withCommercialProfileAPIUser(viewer))
	router.GET("/admin/users/:user_id/commercial-profile", handlers.GetUserProfile)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+jsonInt(other.ID)+"/commercial-profile", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

func TestUserCommercialProfileHandlersOwnerCanInspectUser(t *testing.T) {
	t.Parallel()

	gin.SetMode(gin.TestMode)
	client, ctx, handlers := newCommercialProfileAPITestService(t, "api_commercial_profile_owner")
	owner := createCommercialProfileAPIUser(t, ctx, client, "owner@example.com", true)
	target := createCommercialProfileAPIUser(t, ctx, client, "target@example.com", false)

	router := gin.New()
	router.Use(withCommercialProfileAPIUser(owner))
	router.GET("/admin/users/:user_id/commercial-profile", handlers.GetUserProfile)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+jsonInt(target.ID)+"/commercial-profile?limit=5", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		User struct {
			ID    int    `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		BillingAccount struct {
			OwnerID int `json:"ownerId"`
		} `json:"billingAccount"`
		Filter struct {
			Limit int `json:"limit"`
		} `json:"filter"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, target.ID, body.User.ID)
	require.Equal(t, "target@example.com", body.User.Email)
	require.Equal(t, target.ID, body.BillingAccount.OwnerID)
	require.Equal(t, 5, body.Filter.Limit)
}

func newCommercialProfileAPITestService(t *testing.T, name string) (*ent.Client, context.Context, *UserCommercialProfileHandlers) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	accountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	profileSvc := biz.NewUserCommercialProfileService(biz.UserCommercialProfileServiceParams{
		Ent:                   client,
		BillingAccountService: accountSvc,
	})
	return client, ctx, NewUserCommercialProfileHandlers(UserCommercialProfileHandlersParams{ProfileService: profileSvc})
}

func createCommercialProfileAPIUser(t *testing.T, ctx context.Context, client *ent.Client, email string, isOwner bool) *ent.User {
	t.Helper()

	user, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashed-password").
		SetIsOwner(isOwner).
		Save(ctx)
	require.NoError(t, err)

	return user
}

func withCommercialProfileAPIUser(user *ent.User) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contexts.WithUser(c.Request.Context(), user))
		c.Next()
	}
}

func jsonInt(value int) string {
	return strconv.Itoa(value)
}
