package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

type UserUsageHandlersParams struct {
	fx.In
	Service *biz.UserUsageService
}

type UserUsageHandlers struct {
	Service *biz.UserUsageService
}

func NewUserUsageHandlers(params UserUsageHandlersParams) *UserUsageHandlers {
	return &UserUsageHandlers{Service: params.Service}
}

func (h *UserUsageHandlers) MyRequests(c *gin.Context) { h.listRequests(c, biz.UserUsageScopeMine) }
func (h *UserUsageHandlers) ProjectRequests(c *gin.Context) {
	h.listRequests(c, biz.UserUsageScopeProject)
}
func (h *UserUsageHandlers) MyRequest(c *gin.Context) { h.getRequest(c, biz.UserUsageScopeMine) }
func (h *UserUsageHandlers) ProjectRequest(c *gin.Context) {
	h.getRequest(c, biz.UserUsageScopeProject)
}
func (h *UserUsageHandlers) MyUsage(c *gin.Context)      { h.usage(c, biz.UserUsageScopeMine) }
func (h *UserUsageHandlers) ProjectUsage(c *gin.Context) { h.usage(c, biz.UserUsageScopeProject) }
func (h *UserUsageHandlers) ExportMyRequests(c *gin.Context) {
	h.exportRequests(c, biz.UserUsageScopeMine)
}
func (h *UserUsageHandlers) ExportProjectRequests(c *gin.Context) {
	h.exportRequests(c, biz.UserUsageScopeProject)
}

func (h *UserUsageHandlers) listRequests(c *gin.Context, scope biz.UserUsageScope) {
	viewer, projectID, ok := userUsageRequestContext(c)
	if !ok {
		return
	}
	query, ok := userUsageListQuery(c, projectID, scope)
	if !ok {
		return
	}
	page, err := h.Service.ListRequests(c.Request.Context(), viewer, query)
	if err != nil {
		writeUserUsageError(c, err)
		return
	}
	c.JSON(http.StatusOK, page)
}

func (h *UserUsageHandlers) getRequest(c *gin.Context, scope biz.UserUsageScope) {
	viewer, projectID, ok := userUsageRequestContext(c)
	if !ok {
		return
	}
	requestGUID, err := objects.ParseGUID(c.Param("request_id"))
	if err != nil || requestGUID.Type != ent.TypeRequest || requestGUID.ID <= 0 {
		JSONError(c, http.StatusBadRequest, errors.New("request_id must be a valid Request GUID"))
		return
	}
	detail, err := h.Service.GetRequest(c.Request.Context(), viewer, projectID, scope, requestGUID.ID)
	if err != nil {
		writeUserUsageError(c, err)
		return
	}
	c.JSON(http.StatusOK, detail)
}

func (h *UserUsageHandlers) usage(c *gin.Context, scope biz.UserUsageScope) {
	viewer, projectID, ok := userUsageRequestContext(c)
	if !ok {
		return
	}
	from, to, ok := userUsageTimeRange(c)
	if !ok {
		return
	}
	summary, err := h.Service.Usage(c.Request.Context(), viewer, biz.UserUsageQuery{ProjectID: projectID, Scope: scope, From: from, To: to})
	if err != nil {
		writeUserUsageError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

func (h *UserUsageHandlers) exportRequests(c *gin.Context, scope biz.UserUsageScope) {
	viewer, projectID, ok := userUsageRequestContext(c)
	if !ok {
		return
	}
	query, ok := userUsageListQuery(c, projectID, scope)
	if !ok {
		return
	}
	data, err := h.Service.ExportRequestsCSV(c.Request.Context(), viewer, query)
	if err != nil {
		writeUserUsageError(c, err)
		return
	}
	filename := fmt.Sprintf("requests-%s-%s.csv", scope, time.Now().UTC().Format("20060102-150405"))
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}

func userUsageRequestContext(c *gin.Context) (*ent.User, int, bool) {
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

func userUsageListQuery(c *gin.Context, projectID int, scope biz.UserUsageScope) (biz.UserUsageRequestQuery, bool) {
	offset, err := queryInt(c, "offset", 0)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return biz.UserUsageRequestQuery{}, false
	}
	limit, err := queryInt(c, "limit", 20)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return biz.UserUsageRequestQuery{}, false
	}
	from, to, ok := userUsageOptionalTimeRange(c)
	if !ok {
		return biz.UserUsageRequestQuery{}, false
	}
	return biz.UserUsageRequestQuery{
		ProjectID: projectID, Scope: scope, Offset: offset, Limit: limit,
		Status: c.Query("status"), ModelID: c.Query("modelId"), From: from, To: to,
	}, true
}

func userUsageTimeRange(c *gin.Context) (time.Time, time.Time, bool) {
	from, to, ok := userUsageOptionalTimeRange(c)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	var fromValue, toValue time.Time
	if from != nil {
		fromValue = *from
	}
	if to != nil {
		toValue = *to
	}
	return fromValue, toValue, true
}

func userUsageOptionalTimeRange(c *gin.Context) (*time.Time, *time.Time, bool) {
	parse := func(name string) (*time.Time, error) {
		value := c.Query(name)
		if value == "" {
			return nil, nil
		}
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return nil, fmt.Errorf("%s must be RFC3339", name)
		}
		parsed = parsed.UTC()
		return &parsed, nil
	}
	from, err := parse("from")
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return nil, nil, false
	}
	to, err := parse("to")
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return nil, nil, false
	}
	return from, to, true
}

func queryInt(c *gin.Context, name string, fallback int) (int, error) {
	value := c.Query(name)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	return parsed, nil
}

func writeUserUsageError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, biz.ErrUserUsageDenied):
		JSONError(c, http.StatusForbidden, err)
	case errors.Is(err, biz.ErrUserUsageProjectAdminRequired):
		JSONError(c, http.StatusForbidden, err)
	case errors.Is(err, biz.ErrUserUsageInvalidInput):
		JSONError(c, http.StatusBadRequest, err)
	default:
		JSONError(c, http.StatusInternalServerError, err)
	}
}
