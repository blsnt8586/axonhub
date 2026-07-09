package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/server/biz"
)

type UserCommercialProfileHandlersParams struct {
	fx.In

	ProfileService *biz.UserCommercialProfileService
}

func NewUserCommercialProfileHandlers(params UserCommercialProfileHandlersParams) *UserCommercialProfileHandlers {
	return &UserCommercialProfileHandlers{ProfileService: params.ProfileService}
}

type UserCommercialProfileHandlers struct {
	ProfileService *biz.UserCommercialProfileService
}

func (h *UserCommercialProfileHandlers) GetMyProfile(c *gin.Context) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return
	}

	h.writeProfile(c, viewer.ID)
}

func (h *UserCommercialProfileHandlers) GetUserProfile(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("user_id"))
	if err != nil || userID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("invalid user id"))
		return
	}

	h.writeProfile(c, userID)
}

func (h *UserCommercialProfileHandlers) writeProfile(c *gin.Context, targetUserID int) {
	viewer, ok := contexts.GetUser(c.Request.Context())
	if !ok || viewer == nil {
		JSONError(c, http.StatusUnauthorized, errors.New("missing authenticated user"))
		return
	}

	filter, err := parseCommercialProfileFilter(c)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	profile, err := h.ProfileService.GetProfile(c.Request.Context(), viewer, targetUserID, filter)
	if err != nil {
		switch {
		case errors.Is(err, biz.ErrCommercialProfileDenied):
			JSONError(c, http.StatusForbidden, err)
		default:
			JSONError(c, http.StatusInternalServerError, err)
		}
		return
	}

	c.JSON(http.StatusOK, profile)
}

func parseCommercialProfileFilter(c *gin.Context) (biz.UserCommercialProfileFilter, error) {
	var filter biz.UserCommercialProfileFilter

	from, err := parseOptionalProfileTime(c.Query("from"))
	if err != nil {
		return filter, err
	}
	to, err := parseOptionalProfileTime(c.Query("to"))
	if err != nil {
		return filter, err
	}
	projectID, err := parseOptionalProfileInt(c.Query("projectId"))
	if err != nil {
		return filter, err
	}
	apiKeyID, err := parseOptionalProfileInt(c.Query("apiKeyId"))
	if err != nil {
		return filter, err
	}
	limit, err := parseOptionalProfileLimit(c.Query("limit"))
	if err != nil {
		return filter, err
	}

	filter.From = from
	filter.To = to
	filter.ProjectID = projectID
	filter.APIKeyID = apiKeyID
	filter.ModelID = c.Query("modelId")
	filter.RequestType = c.Query("requestType")
	filter.Limit = limit

	return filter, nil
}

func parseOptionalProfileTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, errors.New("invalid time filter; use RFC3339 or YYYY-MM-DD")
	}

	return &parsed, nil
}

func parseOptionalProfileInt(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return nil, errors.New("numeric filters must be positive integers")
	}

	return &parsed, nil
}

func parseOptionalProfileLimit(value string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, errors.New("limit must be a positive integer")
	}

	return parsed, nil
}
