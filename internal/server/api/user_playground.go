package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/middleware"
)

type UserPlaygroundHandlersParams struct {
	fx.In

	Service    *biz.UserPlaygroundService
	Playground *PlaygroundHandlers
}

type UserPlaygroundHandlers struct {
	Service        *biz.UserPlaygroundService
	ChatCompletion gin.HandlerFunc
}

func NewUserPlaygroundHandlers(params UserPlaygroundHandlersParams) *UserPlaygroundHandlers {
	return &UserPlaygroundHandlers{Service: params.Service, ChatCompletion: params.Playground.ChatCompletion}
}

func (h *UserPlaygroundHandlers) State(c *gin.Context) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return
	}
	state, err := h.Service.State(c.Request.Context(), viewer, projectID, c.ClientIP(), time.Now().UTC())
	if err != nil {
		writeUserPlaygroundError(c, err)
		return
	}
	c.JSON(http.StatusOK, state)
}

func (h *UserPlaygroundHandlers) Chat(c *gin.Context) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return
	}
	keyGUID, err := objects.ParseGUID(c.Query("keyId"))
	if err != nil || keyGUID.Type != ent.TypeAPIKey || keyGUID.ID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("keyId must be a valid APIKey GUID"))
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("failed to read playground request"))
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	var request struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid playground request"))
		return
	}
	prepared, err := h.Service.PrepareChat(c.Request.Context(), viewer, projectID, keyGUID.ID, request.Model, c.ClientIP())
	if err != nil {
		writeUserPlaygroundError(c, err)
		return
	}
	ctx, err := middleware.ContextWithUserOwnedAPIKey(c.Request.Context(), viewer, prepared.APIKey)
	if err != nil {
		JSONError(c, http.StatusUnauthorized, errors.New("invalid authentication context"))
		return
	}
	c.Request = c.Request.WithContext(ctx)
	c.Request.Header.Del("X-Channel-ID")
	c.Request.Header.Del("X-Project-ID")
	query := c.Request.URL.Query()
	query.Del("channel_id")
	query.Del("project_id")
	c.Request.URL.RawQuery = query.Encode()
	c.Set(consumerPlaygroundContextKey, true)
	h.ChatCompletion(c)
}

func writeUserPlaygroundError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, biz.ErrUserPlaygroundDenied):
		JSONError(c, http.StatusForbidden, err)
	case errors.Is(err, biz.ErrUserPlaygroundInvalid):
		JSONError(c, http.StatusBadRequest, err)
	case errors.Is(err, biz.ErrUserPlaygroundKeyDisabled):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"message": err.Error(), "reason": "key_disabled"}})
	case errors.Is(err, biz.ErrUserPlaygroundModelUnavailable):
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"message": err.Error(), "reason": "no_model"}})
	default:
		JSONError(c, http.StatusInternalServerError, err)
	}
}
