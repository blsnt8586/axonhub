package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type UserWorkspaceSummaryHandlersParams struct {
	fx.In

	SummaryService *biz.UserWorkspaceSummaryService
}

type UserWorkspaceSummaryHandlers struct {
	SummaryService *biz.UserWorkspaceSummaryService
}

func NewUserWorkspaceSummaryHandlers(params UserWorkspaceSummaryHandlersParams) *UserWorkspaceSummaryHandlers {
	return &UserWorkspaceSummaryHandlers{SummaryService: params.SummaryService}
}

func (h *UserWorkspaceSummaryHandlers) GetMySummary(c *gin.Context) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return
	}

	projectID := 0
	if raw := c.Query("projectId"); raw != "" {
		parsed, err := objects.ParseGUID(raw)
		if err != nil || parsed.Type != ent.TypeProject || parsed.ID <= 0 {
			JSONError(c, http.StatusBadRequest, errors.New("projectId must be a valid Project GUID"))
			return
		}
		projectID = parsed.ID
	}

	summary, err := h.SummaryService.GetSummary(c.Request.Context(), viewer, projectID, time.Now().UTC())
	if err != nil {
		if errors.Is(err, biz.ErrWorkspaceProjectDenied) {
			JSONError(c, http.StatusForbidden, err)
			return
		}
		JSONError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}
