package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
)

type UserAPIKeyHandlersParams struct {
	fx.In

	Service *biz.UserAPIKeyService
}

type UserAPIKeyHandlers struct {
	Service *biz.UserAPIKeyService
}

type UserAPIKeyCreateRequest struct {
	Name               string                          `json:"name" binding:"required"`
	ExpiresAt          *time.Time                      `json:"expiresAt"`
	IPAllowlist        []string                        `json:"ipAllowlist"`
	AllowedModelIDs    []string                        `json:"allowedModelIds"`
	RequestLimit       *int64                          `json:"requestLimit"`
	RequestLimitWindow biz.UserAPIKeyLimitWindow       `json:"requestLimitWindow"`
	CommercialLimits   *objects.APIKeyCommercialLimits `json:"commercialLimits"`
}

type UserAPIKeyUpdateRequest struct {
	Name                  *string                         `json:"name"`
	Status                *apikey.Status                  `json:"status"`
	ExpiresAt             *time.Time                      `json:"expiresAt"`
	ClearExpiresAt        bool                            `json:"clearExpiresAt"`
	IPAllowlist           *[]string                       `json:"ipAllowlist"`
	AllowedModelIDs       *[]string                       `json:"allowedModelIds"`
	RequestLimit          *int64                          `json:"requestLimit"`
	ClearRequestLimit     bool                            `json:"clearRequestLimit"`
	RequestLimitWindow    biz.UserAPIKeyLimitWindow       `json:"requestLimitWindow"`
	CommercialLimits      *objects.APIKeyCommercialLimits `json:"commercialLimits"`
	ClearCommercialLimits bool                            `json:"clearCommercialLimits"`
}

func NewUserAPIKeyHandlers(params UserAPIKeyHandlersParams) *UserAPIKeyHandlers {
	return &UserAPIKeyHandlers{Service: params.Service}
}

func (h *UserAPIKeyHandlers) List(c *gin.Context) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return
	}
	keys, err := h.Service.List(c.Request.Context(), viewer, projectID)
	if err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"apiKeys": keys})
}

func (h *UserAPIKeyHandlers) Models(c *gin.Context) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return
	}
	models, err := h.Service.Models(c.Request.Context(), viewer, projectID)
	if err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"models": models})
}

func (h *UserAPIKeyHandlers) Create(c *gin.Context) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return
	}
	var req UserAPIKeyCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid user api key request"))
		return
	}
	created, err := h.Service.Create(c.Request.Context(), viewer, biz.UserAPIKeyCreateInput{
		ProjectID: projectID, Name: req.Name, ExpiresAt: req.ExpiresAt, IPAllowlist: req.IPAllowlist,
		AllowedModelIDs: req.AllowedModelIDs, RequestLimit: req.RequestLimit,
		RequestLimitWindow: req.RequestLimitWindow, CommercialLimits: req.CommercialLimits,
	})
	if err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

func (h *UserAPIKeyHandlers) Update(c *gin.Context) {
	viewer, projectID, keyID, ok := userAPIKeyMutationContext(c)
	if !ok {
		return
	}
	var req UserAPIKeyUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		JSONError(c, http.StatusBadRequest, errors.New("invalid user api key request"))
		return
	}
	updated, err := h.Service.Update(c.Request.Context(), viewer, projectID, keyID, biz.UserAPIKeyUpdateInput{
		Name: req.Name, Status: req.Status, ExpiresAt: req.ExpiresAt, ClearExpiresAt: req.ClearExpiresAt,
		IPAllowlist: req.IPAllowlist, AllowedModelIDs: req.AllowedModelIDs, RequestLimit: req.RequestLimit,
		ClearRequestLimit: req.ClearRequestLimit, RequestLimitWindow: req.RequestLimitWindow,
		CommercialLimits: req.CommercialLimits, ClearCommercialLimits: req.ClearCommercialLimits,
	})
	if err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (h *UserAPIKeyHandlers) Rotate(c *gin.Context) {
	viewer, projectID, keyID, ok := userAPIKeyMutationContext(c)
	if !ok {
		return
	}
	rotated, err := h.Service.Rotate(c.Request.Context(), viewer, projectID, keyID)
	if err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.JSON(http.StatusOK, rotated)
}

func (h *UserAPIKeyHandlers) Archive(c *gin.Context) {
	viewer, projectID, keyID, ok := userAPIKeyMutationContext(c)
	if !ok {
		return
	}
	if err := h.Service.Archive(c.Request.Context(), viewer, projectID, keyID); err != nil {
		writeUserAPIKeyError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func userAPIKeyRequestContext(c *gin.Context) (*ent.User, int, bool) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return nil, 0, false
	}
	projectGUID, err := objects.ParseGUID(c.Query("projectId"))
	if err != nil || projectGUID.Type != ent.TypeProject || projectGUID.ID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("projectId must be a valid Project GUID"))
		return nil, 0, false
	}
	return viewer, projectGUID.ID, true
}

func userAPIKeyMutationContext(c *gin.Context) (*ent.User, int, int, bool) {
	viewer, projectID, ok := userAPIKeyRequestContext(c)
	if !ok {
		return nil, 0, 0, false
	}
	keyGUID, err := objects.ParseGUID(c.Query("keyId"))
	if err != nil || keyGUID.Type != ent.TypeAPIKey || keyGUID.ID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("keyId must be a valid APIKey GUID"))
		return nil, 0, 0, false
	}
	return viewer, projectID, keyGUID.ID, true
}

func writeUserAPIKeyError(c *gin.Context, err error) {
	if errors.Is(err, biz.ErrUserAPIKeyDenied) {
		JSONError(c, http.StatusForbidden, err)
		return
	}
	if errors.Is(err, biz.ErrUserAPIKeyInvalidInput) {
		JSONError(c, http.StatusBadRequest, err)
		return
	}
	if coded, ok := xerrors.IsCodedError(err); ok && coded.Code == xerrors.ErrCodeDuplicateName {
		JSONError(c, http.StatusConflict, err)
		return
	}
	JSONError(c, http.StatusInternalServerError, err)
}
