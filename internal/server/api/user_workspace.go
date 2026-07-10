package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
)

type UserWorkspaceHandlersParams struct {
	fx.In

	WorkspaceService *biz.UserWorkspaceService
}

type UserWorkspaceHandlers struct {
	WorkspaceService *biz.UserWorkspaceService
}

type CreateUserWorkspaceRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

type UserWorkspaceSettingsResponse struct {
	AllowSelfServiceCreation bool `json:"allowSelfServiceCreation"`
	MaxWorkspacesPerUser     int  `json:"maxWorkspacesPerUser"`
}

func NewUserWorkspaceHandlers(params UserWorkspaceHandlersParams) *UserWorkspaceHandlers {
	return &UserWorkspaceHandlers{WorkspaceService: params.WorkspaceService}
}

func (h *UserWorkspaceHandlers) List(c *gin.Context) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return
	}

	result, err := h.WorkspaceService.List(c.Request.Context(), viewer)
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *UserWorkspaceHandlers) Create(c *gin.Context) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return
	}

	var req CreateUserWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("workspace name is required"))
		return
	}

	created, err := h.WorkspaceService.Create(c.Request.Context(), viewer, biz.CreateUserWorkspaceInput{
		Name: req.Name, Description: req.Description,
	})
	if err != nil {
		codedErr, isCoded := xerrors.IsCodedError(err)
		switch {
		case errors.Is(err, biz.ErrWorkspaceCreationDisabled), errors.Is(err, biz.ErrWorkspaceLimitReached):
			JSONError(c, http.StatusForbidden, err)
		case errors.Is(err, biz.ErrWorkspaceInvalidName):
			JSONError(c, http.StatusBadRequest, err)
		case isCoded && codedErr.Code == xerrors.ErrCodeDuplicateName:
			JSONError(c, http.StatusConflict, err)
		default:
			JSONError(c, http.StatusInternalServerError, err)
		}
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *UserWorkspaceHandlers) GetSettings(c *gin.Context) {
	if !canReadSettings(c.Request.Context()) {
		JSONError(c, http.StatusForbidden, errors.New("permission denied: requires read settings scope"))
		return
	}
	settings, err := h.WorkspaceService.Settings(c.Request.Context())
	if err != nil {
		JSONError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, UserWorkspaceSettingsResponse{
		AllowSelfServiceCreation: settings.AllowSelfServiceCreation,
		MaxWorkspacesPerUser:     settings.MaxWorkspacesPerUser,
	})
}

func (h *UserWorkspaceHandlers) UpdateSettings(c *gin.Context) {
	if !canWriteSettings(c.Request.Context()) {
		JSONError(c, http.StatusForbidden, errors.New("permission denied: requires write settings scope"))
		return
	}
	var req UserWorkspaceSettingsResponse
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid request format"))
		return
	}
	settings := biz.UserWorkspaceSettings{
		AllowSelfServiceCreation: req.AllowSelfServiceCreation,
		MaxWorkspacesPerUser:     req.MaxWorkspacesPerUser,
	}
	if err := h.WorkspaceService.SetSettings(c.Request.Context(), settings); err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}
	h.GetSettings(c)
}
