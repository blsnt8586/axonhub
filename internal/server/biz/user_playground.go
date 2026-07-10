package biz

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
)

type UserPlaygroundBlockReason string

const (
	UserPlaygroundBlockReasonNone                UserPlaygroundBlockReason = "none"
	UserPlaygroundBlockReasonNoModel             UserPlaygroundBlockReason = "no_model"
	UserPlaygroundBlockReasonNoEnabledKey        UserPlaygroundBlockReason = "no_enabled_key"
	UserPlaygroundBlockReasonKeyDisabled         UserPlaygroundBlockReason = "key_disabled"
	UserPlaygroundBlockReasonKeyExpired          UserPlaygroundBlockReason = "key_expired"
	UserPlaygroundBlockReasonKeyRestricted       UserPlaygroundBlockReason = "key_restricted"
	UserPlaygroundBlockReasonBalanceInsufficient UserPlaygroundBlockReason = "balance_insufficient"
	UserPlaygroundBlockReasonAccountUnavailable  UserPlaygroundBlockReason = "account_unavailable"
)

type UserPlaygroundKeyBlockReason string

const (
	UserPlaygroundKeyBlockReasonNone         UserPlaygroundKeyBlockReason = "none"
	UserPlaygroundKeyBlockReasonDisabled     UserPlaygroundKeyBlockReason = "disabled"
	UserPlaygroundKeyBlockReasonExpired      UserPlaygroundKeyBlockReason = "expired"
	UserPlaygroundKeyBlockReasonIPNotAllowed UserPlaygroundKeyBlockReason = "ip_not_allowed"
)

type UserPlaygroundModelAvailability string

const (
	UserPlaygroundModelAvailabilityAvailable UserPlaygroundModelAvailability = "available"
)

type UserPlaygroundPriceRule struct {
	Scope   string `json:"scope"`
	Pattern string `json:"pattern"`
}

type UserPlaygroundModel struct {
	ModelID      string                          `json:"modelId"`
	DisplayName  string                          `json:"displayName"`
	Modality     string                          `json:"modality"`
	Availability UserPlaygroundModelAvailability `json:"availability"`
	Currency     string                          `json:"currency,omitempty"`
	Price        *objects.ModelPrice             `json:"price,omitempty"`
	PriceRule    *UserPlaygroundPriceRule        `json:"priceRule,omitempty"`
}

type UserPlaygroundAPIKey struct {
	ID              string                       `json:"id"`
	Name            string                       `json:"name"`
	Status          apikey.Status                `json:"status"`
	Usable          bool                         `json:"usable"`
	BlockReason     UserPlaygroundKeyBlockReason `json:"blockReason"`
	AllowedModelIDs []string                     `json:"allowedModelIds"`
}

type UserPlaygroundState struct {
	ProjectID   string                    `json:"projectId"`
	CanSend     bool                      `json:"canSend"`
	BlockReason UserPlaygroundBlockReason `json:"blockReason"`
	APIKeys     []UserPlaygroundAPIKey    `json:"apiKeys"`
	Models      []UserPlaygroundModel     `json:"models"`
}

type UserPlaygroundPreparedChat struct {
	ProjectID int
	APIKey    *ent.APIKey
}

type UserPlaygroundServiceParams struct {
	fx.In

	Ent                     *ent.Client
	UserAPIKeyService       *UserAPIKeyService
	WorkspaceSummaryService *UserWorkspaceSummaryService
	PricingService          *PricingService
	AdmissionService        *AdmissionService
}

type UserPlaygroundService struct {
	*AbstractService

	userAPIKeys      *UserAPIKeyService
	workspaceSummary *UserWorkspaceSummaryService
	pricing          *PricingService
	admission        *AdmissionService
}

func NewUserPlaygroundService(params UserPlaygroundServiceParams) *UserPlaygroundService {
	return &UserPlaygroundService{
		AbstractService:  &AbstractService{db: params.Ent},
		userAPIKeys:      params.UserAPIKeyService,
		workspaceSummary: params.WorkspaceSummaryService,
		pricing:          params.PricingService,
		admission:        params.AdmissionService,
	}
}

func (s *UserPlaygroundService) State(
	ctx context.Context,
	viewer *ent.User,
	projectID int,
	clientIP string,
	now time.Time,
) (*UserPlaygroundState, error) {
	bypassCtx, err := s.userAPIKeys.authorizedWorkspaceContext(ctx, viewer, projectID, "user-playground-state")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserPlaygroundDenied, err)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	projectRow, err := s.entFromContext(bypassCtx).Project.Get(bypassCtx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to load playground workspace: %w", err)
	}
	available, err := s.workspaceSummary.availableModels(bypassCtx, projectRow)
	if err != nil {
		return nil, err
	}
	models, err := s.consumerModels(bypassCtx, projectID, available)
	if err != nil {
		return nil, err
	}
	rows, err := s.entFromContext(bypassCtx).APIKey.Query().Where(
		apikey.UserIDEQ(viewer.ID),
		apikey.ProjectIDEQ(projectID),
		apikey.TypeIn(apikey.TypeUser, apikey.TypePersonal),
		apikey.StatusNEQ(apikey.StatusArchived),
	).Order(ent.Desc(apikey.FieldCreatedAt)).All(bypassCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to list playground api keys: %w", err)
	}

	state := &UserPlaygroundState{
		ProjectID: workspaceObjectGUID(projectID, ent.TypeProject),
		APIKeys:   make([]UserPlaygroundAPIKey, 0, len(rows)),
		Models:    models,
	}
	usableKeys := make([]*ent.APIKey, 0, len(rows))
	for _, row := range rows {
		keyState := userPlaygroundKeyState(row, clientIP, now)
		state.APIKeys = append(state.APIKeys, keyState)
		if keyState.Usable {
			row.Edges.Project = projectRow
			usableKeys = append(usableKeys, row)
		}
	}

	state.BlockReason = s.blockReason(bypassCtx, projectID, rows, usableKeys, playgroundChatModels(models), now)
	state.CanSend = state.BlockReason == UserPlaygroundBlockReasonNone
	return state, nil
}

func (s *UserPlaygroundService) PrepareChat(
	ctx context.Context,
	viewer *ent.User,
	projectID, keyID int,
	modelID, clientIP string,
) (*UserPlaygroundPreparedChat, error) {
	bypassCtx, err := s.userAPIKeys.authorizedWorkspaceContext(ctx, viewer, projectID, "user-playground-chat")
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserPlaygroundDenied, err)
	}
	key, err := s.entFromContext(bypassCtx).APIKey.Query().Where(
		apikey.IDEQ(keyID),
		apikey.UserIDEQ(viewer.ID),
		apikey.ProjectIDEQ(projectID),
		apikey.TypeIn(apikey.TypeUser, apikey.TypePersonal),
		apikey.StatusNEQ(apikey.StatusArchived),
	).WithProject().Only(bypassCtx)
	if ent.IsNotFound(err) {
		return nil, ErrUserPlaygroundDenied
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load playground api key: %w", err)
	}
	if err := ValidateAPIKeyRequestAccess(key, clientIP, time.Now().UTC()); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUserPlaygroundKeyDisabled, err)
	}
	modelID = normalizeModelID(modelID)
	if modelID == "" {
		return nil, fmt.Errorf("%w: model is required", ErrUserPlaygroundInvalid)
	}
	available, err := s.workspaceSummary.availableModels(bypassCtx, key.Edges.Project)
	if err != nil {
		return nil, err
	}
	if _, ok := available[modelID]; !ok || !apiKeyAllowsModel(key, modelID) {
		return nil, ErrUserPlaygroundModelUnavailable
	}
	configured, err := s.entFromContext(bypassCtx).Model.Query().Where(model.ModelIDEQ(modelID)).Only(bypassCtx)
	if err == nil && configured.Type != model.TypeChat {
		return nil, ErrUserPlaygroundModelUnavailable
	}
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to validate playground model modality: %w", err)
	}
	return &UserPlaygroundPreparedChat{ProjectID: projectID, APIKey: key}, nil
}

func (s *UserPlaygroundService) consumerModels(
	ctx context.Context,
	projectID int,
	available map[string]struct{},
) ([]UserPlaygroundModel, error) {
	modelIDs := make([]string, 0, len(available))
	for modelID := range available {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	configured := map[string]*ent.Model{}
	if len(modelIDs) > 0 {
		rows, err := s.entFromContext(ctx).Model.Query().Where(model.ModelIDIn(modelIDs...)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load consumer model metadata: %w", err)
		}
		for _, row := range rows {
			configured[row.ModelID] = row
		}
	}

	result := make([]UserPlaygroundModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		item := UserPlaygroundModel{
			ModelID: modelID, DisplayName: modelID, Modality: "chat",
			Availability: UserPlaygroundModelAvailabilityAvailable,
		}
		if row := configured[modelID]; row != nil {
			item.DisplayName = row.Name
			item.Modality = string(row.Type)
		}
		rule, err := s.pricing.FindSellPrice(ctx, projectID, modelID)
		if err == nil {
			price := rule.Price
			item.Currency = rule.Currency
			item.Price = &price
			item.PriceRule = &UserPlaygroundPriceRule{Scope: string(rule.ScopeType), Pattern: rule.ModelPattern}
		} else if !errors.Is(err, ErrBillingPriceNotFound) {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *UserPlaygroundService) blockReason(
	ctx context.Context,
	projectID int,
	allKeys, usableKeys []*ent.APIKey,
	models []UserPlaygroundModel,
	now time.Time,
) UserPlaygroundBlockReason {
	if len(models) == 0 {
		return UserPlaygroundBlockReasonNoModel
	}
	if len(allKeys) == 0 {
		return UserPlaygroundBlockReasonNoEnabledKey
	}
	if len(usableKeys) == 0 {
		for _, key := range allKeys {
			if key.Status != apikey.StatusEnabled {
				return UserPlaygroundBlockReasonKeyDisabled
			}
			if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
				return UserPlaygroundBlockReasonKeyExpired
			}
		}
		return UserPlaygroundBlockReasonKeyRestricted
	}

	if s.admission == nil {
		for _, key := range usableKeys {
			for _, modelItem := range models {
				if apiKeyAllowsModel(key, modelItem.ModelID) {
					return UserPlaygroundBlockReasonNone
				}
			}
		}
		return UserPlaygroundBlockReasonKeyRestricted
	}
	sawInsufficientBalance := false
	sawAllowedModel := false
	for _, key := range usableKeys {
		subject, ok := s.admission.BillingSubjectForAPIKey(key, projectID)
		if !ok {
			return UserPlaygroundBlockReasonNone
		}
		for _, modelItem := range models {
			if !apiKeyAllowsModel(key, modelItem.ModelID) {
				continue
			}
			sawAllowedModel = true
			decision, err := s.admission.Check(ctx, AdmissionCheckInput{
				Subject: subject, ProjectID: projectID, ModelID: modelItem.ModelID, APIKey: key,
			})
			if decision.Allowed {
				return UserPlaygroundBlockReasonNone
			}
			if errors.Is(err, ErrInsufficientBalance) || decision.Code == AdmissionCodeInsufficientBalance || decision.Code == AdmissionCodeSubscriptionExhausted {
				sawInsufficientBalance = true
			}
		}
	}
	if sawInsufficientBalance {
		return UserPlaygroundBlockReasonBalanceInsufficient
	}
	if !sawAllowedModel {
		return UserPlaygroundBlockReasonKeyRestricted
	}
	return UserPlaygroundBlockReasonAccountUnavailable
}

func playgroundChatModels(models []UserPlaygroundModel) []UserPlaygroundModel {
	result := make([]UserPlaygroundModel, 0, len(models))
	for _, item := range models {
		if item.Modality == "chat" {
			result = append(result, item)
		}
	}
	return result
}

func userPlaygroundKeyState(key *ent.APIKey, clientIP string, now time.Time) UserPlaygroundAPIKey {
	state := UserPlaygroundAPIKey{
		ID: workspaceObjectGUID(key.ID, ent.TypeAPIKey), Name: key.Name, Status: key.Status,
		BlockReason: UserPlaygroundKeyBlockReasonNone,
	}
	if profile := key.GetActiveProfile(); profile != nil {
		state.AllowedModelIDs = slices.Clone(profile.ModelIDs)
	}
	if key.Status != apikey.StatusEnabled {
		state.BlockReason = UserPlaygroundKeyBlockReasonDisabled
		return state
	}
	if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
		state.BlockReason = UserPlaygroundKeyBlockReasonExpired
		return state
	}
	if err := ValidateAPIKeyRequestAccess(key, clientIP, now); err != nil {
		state.BlockReason = UserPlaygroundKeyBlockReasonIPNotAllowed
		return state
	}
	state.Usable = true
	return state
}

func apiKeyAllowsModel(key *ent.APIKey, modelID string) bool {
	profile := key.GetActiveProfile()
	return profile == nil || len(profile.ModelIDs) == 0 || slices.Contains(profile.ModelIDs, modelID)
}

func normalizeModelID(modelID string) string {
	for len(modelID) > 0 && (modelID[0] == ' ' || modelID[0] == '\t' || modelID[0] == '\n' || modelID[0] == '\r') {
		modelID = modelID[1:]
	}
	for len(modelID) > 0 {
		last := modelID[len(modelID)-1]
		if last != ' ' && last != '\t' && last != '\n' && last != '\r' {
			break
		}
		modelID = modelID[:len(modelID)-1]
	}
	return modelID
}
