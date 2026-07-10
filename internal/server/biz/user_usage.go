package biz

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagedailyaggregate"
	"github.com/looplj/axonhub/internal/ent/usagehourlyaggregate"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/scopes"
)

type UserUsageScope string

const (
	UserUsageScopeMine    UserUsageScope = "mine"
	UserUsageScopeProject UserUsageScope = "project"
)

type UserUsageServiceParams struct {
	fx.In

	Ent               *ent.Client
	UserAPIKeyService *UserAPIKeyService
	RequestService    *RequestService
}

type UserUsageService struct {
	*AbstractService
	userAPIKeys *UserAPIKeyService
	requests    *RequestService
}

func NewUserUsageService(params UserUsageServiceParams) *UserUsageService {
	return &UserUsageService{
		AbstractService: &AbstractService{db: params.Ent},
		userAPIKeys:     params.UserAPIKeyService,
		requests:        params.RequestService,
	}
}

type UserUsageRequestQuery struct {
	ProjectID int
	Scope     UserUsageScope
	Offset    int
	Limit     int
	Status    string
	ModelID   string
	From      *time.Time
	To        *time.Time
}

type UserUsageRequestPage struct {
	Items  []UserUsageRequest `json:"items"`
	Total  int                `json:"total"`
	Offset int                `json:"offset"`
	Limit  int                `json:"limit"`
}

type UserUsageAPIKey struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserUsageRequest struct {
	ID                  string           `json:"id"`
	CreatedAt           time.Time        `json:"createdAt"`
	UpdatedAt           time.Time        `json:"updatedAt"`
	APIKey              *UserUsageAPIKey `json:"apiKey,omitempty"`
	Source              string           `json:"source"`
	ModelID             string           `json:"modelId"`
	Status              string           `json:"status"`
	Format              string           `json:"format"`
	Stream              bool             `json:"stream"`
	LatencyMs           *int64           `json:"latencyMs,omitempty"`
	FirstTokenLatencyMs *int64           `json:"firstTokenLatencyMs,omitempty"`
	PromptTokens        int64            `json:"promptTokens"`
	CompletionTokens    int64            `json:"completionTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	ChargeAmountMicros  int64            `json:"chargeAmountMicros"`
	Currency            string           `json:"currency,omitempty"`
	BillingRecordID     string           `json:"billingRecordId,omitempty"`
	ChannelID           string           `json:"-"`
	UpstreamAccountID   string           `json:"-"`
}

type UserUsageBillingProjection struct {
	BillingRecordID     string              `json:"billingRecordId"`
	LedgerTransactionID string              `json:"ledgerTransactionId,omitempty"`
	ChargeAmountMicros  int64               `json:"chargeAmountMicros"`
	Currency            string              `json:"currency"`
	Status              string              `json:"status"`
	PriceReferenceID    string              `json:"priceReferenceId"`
	PriceSnapshot       *objects.ModelPrice `json:"priceSnapshot,omitempty"`
	ChargeItems         []objects.CostItem  `json:"chargeItems"`
}

type UserUsageRequestDetail struct {
	UserUsageRequest
	RequestBody  objects.JSONRawMessage      `json:"requestBody,omitempty"`
	ResponseBody objects.JSONRawMessage      `json:"responseBody,omitempty"`
	Billing      *UserUsageBillingProjection `json:"billing,omitempty"`
}

type UserUsageQuery struct {
	ProjectID int
	Scope     UserUsageScope
	From      time.Time
	To        time.Time
}

type UserUsageSeriesPoint struct {
	BucketStart        time.Time `json:"bucketStart"`
	RequestCount       int64     `json:"requestCount"`
	SuccessCount       int64     `json:"successCount"`
	ErrorCount         int64     `json:"errorCount"`
	TotalTokens        int64     `json:"totalTokens"`
	ChargeAmountMicros int64     `json:"chargeAmountMicros"`
}

type UserUsageModelTotal struct {
	ModelID            string `json:"modelId"`
	RequestCount       int64  `json:"requestCount"`
	TotalTokens        int64  `json:"totalTokens"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
}

type UserUsageSummary struct {
	Scope              UserUsageScope         `json:"scope"`
	ProjectID          string                 `json:"projectId"`
	From               time.Time              `json:"from"`
	To                 time.Time              `json:"to"`
	Granularity        string                 `json:"granularity"`
	Currency           string                 `json:"currency"`
	RequestCount       int64                  `json:"requestCount"`
	SuccessCount       int64                  `json:"successCount"`
	ErrorCount         int64                  `json:"errorCount"`
	PromptTokens       int64                  `json:"promptTokens"`
	CompletionTokens   int64                  `json:"completionTokens"`
	TotalTokens        int64                  `json:"totalTokens"`
	ChargeAmountMicros int64                  `json:"chargeAmountMicros"`
	Series             []UserUsageSeriesPoint `json:"series"`
	Models             []UserUsageModelTotal  `json:"models"`
}

func (s *UserUsageService) ListRequests(ctx context.Context, viewer *ent.User, input UserUsageRequestQuery) (*UserUsageRequestPage, error) {
	ctx, scope, err := s.authorizedContext(ctx, viewer, input.ProjectID, input.Scope)
	if err != nil {
		return nil, err
	}
	input.Scope = scope
	if input.Offset < 0 {
		return nil, fmt.Errorf("%w: offset must not be negative", ErrUserUsageInvalidInput)
	}
	if input.Limit <= 0 {
		input.Limit = 20
	}
	if input.Limit > 100 {
		input.Limit = 100
	}
	if input.Status != "" {
		if err := request.StatusValidator(request.Status(input.Status)); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUserUsageInvalidInput, err)
		}
	}
	if input.From != nil && input.To != nil && !input.To.After(*input.From) {
		return nil, fmt.Errorf("%w: to must be after from", ErrUserUsageInvalidInput)
	}
	query := s.requestQuery(ctx, viewer.ID, input)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count user requests: %w", err)
	}
	rows, err := query.
		WithAPIKey().
		WithUsageLogs(func(q *ent.UsageLogQuery) { q.WithUsageBillingRecords() }).
		Order(ent.Desc(request.FieldCreatedAt), ent.Desc(request.FieldID)).
		Offset(input.Offset).
		Limit(input.Limit).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list user requests: %w", err)
	}
	items := make([]UserUsageRequest, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectUserUsageRequest(row))
	}
	return &UserUsageRequestPage{Items: items, Total: total, Offset: input.Offset, Limit: input.Limit}, nil
}

func (s *UserUsageService) GetRequest(ctx context.Context, viewer *ent.User, projectID int, scope UserUsageScope, requestID int) (*UserUsageRequestDetail, error) {
	ctx, scope, err := s.authorizedContext(ctx, viewer, projectID, scope)
	if err != nil {
		return nil, err
	}
	input := UserUsageRequestQuery{ProjectID: projectID, Scope: scope}
	row, err := s.requestQuery(ctx, viewer.ID, input).
		Where(request.IDEQ(requestID)).
		WithAPIKey().
		WithUsageLogs(func(q *ent.UsageLogQuery) { q.WithUsageBillingRecords() }).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrUserUsageDenied
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user request: %w", err)
	}
	requestBody, responseBody := row.RequestBody, row.ResponseBody
	if s.requests != nil {
		requestBody, err = s.requests.LoadRequestBody(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("failed to load user request body: %w", err)
		}
		responseBody, err = s.requests.LoadResponseBody(ctx, row)
		if err != nil {
			return nil, fmt.Errorf("failed to load user response body: %w", err)
		}
	}
	base := projectUserUsageRequest(row)
	detail := &UserUsageRequestDetail{UserUsageRequest: base, RequestBody: requestBody, ResponseBody: responseBody}
	if record := requestBillingRecord(row); record != nil {
		price := record.PriceSnapshot
		detail.Billing = &UserUsageBillingProjection{
			BillingRecordID:    workspaceObjectGUID(record.ID, ent.TypeUsageBillingRecord),
			ChargeAmountMicros: record.ChargeAmountMicros, Currency: record.Currency,
			Status: string(record.Status), PriceReferenceID: record.PriceReferenceID,
			PriceSnapshot: &price, ChargeItems: slices.Clone(record.ChargeItems),
		}
		if record.LedgerTransactionID > 0 {
			detail.Billing.LedgerTransactionID = workspaceObjectGUID(record.LedgerTransactionID, ent.TypeLedgerTransaction)
		}
	}
	return detail, nil
}

func (s *UserUsageService) Usage(ctx context.Context, viewer *ent.User, input UserUsageQuery) (*UserUsageSummary, error) {
	ctx, scope, err := s.authorizedContext(ctx, viewer, input.ProjectID, input.Scope)
	if err != nil {
		return nil, err
	}
	input.Scope = scope
	if input.To.IsZero() {
		input.To = time.Now().UTC()
	}
	if input.From.IsZero() {
		input.From = input.To.AddDate(0, 0, -30)
	}
	input.From = input.From.UTC()
	input.To = input.To.UTC()
	if !input.To.After(input.From) {
		return nil, fmt.Errorf("%w: to must be after from", ErrUserUsageInvalidInput)
	}

	summary := &UserUsageSummary{
		Scope: scope, ProjectID: workspaceObjectGUID(input.ProjectID, ent.TypeProject), From: input.From, To: input.To,
		Currency: "CNY", Series: []UserUsageSeriesPoint{}, Models: []UserUsageModelTotal{},
	}
	points := map[time.Time]*UserUsageSeriesPoint{}
	models := map[string]*UserUsageModelTotal{}
	if input.To.Sub(input.From) <= 48*time.Hour {
		summary.Granularity = "hour"
		rows, err := s.entFromContext(ctx).UsageHourlyAggregate.Query().Where(
			usagehourlyaggregate.ProjectIDEQ(input.ProjectID),
			usagehourlyaggregate.BucketStartGTE(input.From), usagehourlyaggregate.BucketStartLTE(input.To),
		).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query hourly user usage: %w", err)
		}
		for _, row := range rows {
			if scope == UserUsageScopeMine && row.UserID != viewer.ID {
				continue
			}
			addUserUsageAggregate(summary, points, models, row.BucketStart, row.ModelID, row.Currency, row.RequestCount, row.SuccessCount, row.ErrorCount, row.PromptTokens, row.CompletionTokens, row.TotalTokens, row.UserChargeMicros)
		}
	} else {
		summary.Granularity = "day"
		rows, err := s.entFromContext(ctx).UsageDailyAggregate.Query().Where(
			usagedailyaggregate.ProjectIDEQ(input.ProjectID),
			usagedailyaggregate.BucketStartGTE(input.From), usagedailyaggregate.BucketStartLTE(input.To),
		).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query daily user usage: %w", err)
		}
		for _, row := range rows {
			if scope == UserUsageScopeMine && row.UserID != viewer.ID {
				continue
			}
			addUserUsageAggregate(summary, points, models, row.BucketStart, row.ModelID, row.Currency, row.RequestCount, row.SuccessCount, row.ErrorCount, row.PromptTokens, row.CompletionTokens, row.TotalTokens, row.UserChargeMicros)
		}
	}
	for _, point := range points {
		summary.Series = append(summary.Series, *point)
	}
	for _, item := range models {
		summary.Models = append(summary.Models, *item)
	}
	sort.Slice(summary.Series, func(i, j int) bool { return summary.Series[i].BucketStart.Before(summary.Series[j].BucketStart) })
	sort.Slice(summary.Models, func(i, j int) bool {
		return summary.Models[i].ChargeAmountMicros > summary.Models[j].ChargeAmountMicros
	})
	return summary, nil
}

func (s *UserUsageService) ExportRequestsCSV(ctx context.Context, viewer *ent.User, input UserUsageRequestQuery) ([]byte, error) {
	input.Offset = 0
	input.Limit = 100
	var all []UserUsageRequest
	for {
		page, err := s.ListRequests(ctx, viewer, input)
		if err != nil {
			return nil, err
		}
		all = append(all, page.Items...)
		if len(all) >= page.Total || len(page.Items) == 0 || len(all) >= 10000 {
			break
		}
		input.Offset += len(page.Items)
	}
	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)
	_ = writer.Write([]string{"request_id", "created_at", "model_id", "source", "status", "api_key", "prompt_tokens", "completion_tokens", "total_tokens", "charge_micros", "currency"})
	for _, item := range all {
		keyName := ""
		if item.APIKey != nil {
			keyName = item.APIKey.Name
		}
		_ = writer.Write([]string{item.ID, item.CreatedAt.Format(time.RFC3339Nano), item.ModelID, item.Source, item.Status, keyName,
			strconv.FormatInt(item.PromptTokens, 10), strconv.FormatInt(item.CompletionTokens, 10), strconv.FormatInt(item.TotalTokens, 10), strconv.FormatInt(item.ChargeAmountMicros, 10), item.Currency})
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("failed to export user requests: %w", err)
	}
	return buffer.Bytes(), nil
}

func (s *UserUsageService) authorizedContext(ctx context.Context, viewer *ent.User, projectID int, scope UserUsageScope) (context.Context, UserUsageScope, error) {
	bypassCtx, err := s.userAPIKeys.authorizedWorkspaceContext(ctx, viewer, projectID, "user-usage")
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrUserUsageDenied, err)
	}
	if scope == "" {
		scope = UserUsageScopeMine
	}
	if scope != UserUsageScopeMine && scope != UserUsageScopeProject {
		return nil, "", ErrUserUsageInvalidInput
	}
	if scope == UserUsageScopeProject {
		allowed, err := s.projectAdminAllowed(bypassCtx, viewer, projectID)
		if err != nil {
			return nil, "", err
		}
		if !allowed {
			return nil, "", ErrUserUsageProjectAdminRequired
		}
	}
	return bypassCtx, scope, nil
}

func (s *UserUsageService) projectAdminAllowed(ctx context.Context, viewer *ent.User, projectID int) (bool, error) {
	if viewer.IsOwner || scopes.HasSystemScope(viewer, scopes.ScopeReadRequests) {
		return true, nil
	}
	membership, err := s.entFromContext(ctx).UserProject.Query().Where(userproject.UserIDEQ(viewer.ID), userproject.ProjectIDEQ(projectID)).Only(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to load project usage membership: %w", err)
	}
	if membership.IsOwner || slices.Contains(membership.Scopes, string(scopes.ScopeReadRequests)) {
		return true, nil
	}
	for _, role := range viewer.Edges.Roles {
		if role.ProjectID != nil && *role.ProjectID == projectID && slices.Contains(role.Scopes, string(scopes.ScopeReadRequests)) {
			return true, nil
		}
	}
	return false, nil
}

func (s *UserUsageService) requestQuery(ctx context.Context, userID int, input UserUsageRequestQuery) *ent.RequestQuery {
	query := s.entFromContext(ctx).Request.Query().Where(request.ProjectIDEQ(input.ProjectID))
	if input.Scope == UserUsageScopeMine {
		query.Where(request.HasAPIKeyWith(apikey.UserIDEQ(userID)))
	}
	if input.Status != "" {
		query.Where(request.StatusEQ(request.Status(input.Status)))
	}
	if value := strings.TrimSpace(input.ModelID); value != "" {
		query.Where(request.ModelIDContainsFold(value))
	}
	if input.From != nil {
		query.Where(request.CreatedAtGTE(input.From.UTC()))
	}
	if input.To != nil {
		query.Where(request.CreatedAtLTE(input.To.UTC()))
	}
	return query
}

func projectUserUsageRequest(row *ent.Request) UserUsageRequest {
	result := UserUsageRequest{
		ID: workspaceObjectGUID(row.ID, ent.TypeRequest), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		Source: string(row.Source), ModelID: row.ModelID, Status: string(row.Status), Format: row.Format,
		Stream: row.Stream, LatencyMs: row.MetricsLatencyMs, FirstTokenLatencyMs: row.MetricsFirstTokenLatencyMs,
	}
	if key := row.Edges.APIKey; key != nil {
		result.APIKey = &UserUsageAPIKey{ID: workspaceObjectGUID(key.ID, ent.TypeAPIKey), Name: key.Name}
	}
	if len(row.Edges.UsageLogs) > 0 {
		usage := row.Edges.UsageLogs[0]
		result.PromptTokens, result.CompletionTokens, result.TotalTokens = usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens
	}
	if record := requestBillingRecord(row); record != nil {
		result.ChargeAmountMicros, result.Currency = record.ChargeAmountMicros, record.Currency
		result.BillingRecordID = workspaceObjectGUID(record.ID, ent.TypeUsageBillingRecord)
	}
	return result
}

func requestBillingRecord(row *ent.Request) *ent.UsageBillingRecord {
	for _, usage := range row.Edges.UsageLogs {
		if len(usage.Edges.UsageBillingRecords) > 0 {
			return usage.Edges.UsageBillingRecords[0]
		}
	}
	return nil
}

func addUserUsageAggregate(summary *UserUsageSummary, points map[time.Time]*UserUsageSeriesPoint, models map[string]*UserUsageModelTotal, bucket time.Time, modelID, currency string, requests, success, failed, prompt, completion, tokens, charge int64) {
	if currency != "" {
		summary.Currency = currency
	}
	summary.RequestCount += requests
	summary.SuccessCount += success
	summary.ErrorCount += failed
	summary.PromptTokens += prompt
	summary.CompletionTokens += completion
	summary.TotalTokens += tokens
	summary.ChargeAmountMicros += charge
	point := points[bucket]
	if point == nil {
		point = &UserUsageSeriesPoint{BucketStart: bucket}
		points[bucket] = point
	}
	point.RequestCount += requests
	point.SuccessCount += success
	point.ErrorCount += failed
	point.TotalTokens += tokens
	point.ChargeAmountMicros += charge
	item := models[modelID]
	if item == nil {
		item = &UserUsageModelTotal{ModelID: modelID}
		models[modelID] = item
	}
	item.RequestCount += requests
	item.TotalTokens += tokens
	item.ChargeAmountMicros += charge
}
