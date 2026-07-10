package biz

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
)

type UserAPIKeyLimitWindow string

const (
	UserAPIKeyLimitWindowAllTime UserAPIKeyLimitWindow = "all_time"
	UserAPIKeyLimitWindowMinute  UserAPIKeyLimitWindow = "minute"
	UserAPIKeyLimitWindowHour    UserAPIKeyLimitWindow = "hour"
	UserAPIKeyLimitWindowDay     UserAPIKeyLimitWindow = "day"
)

type UserAPIKeyServiceParams struct {
	fx.In

	Ent                     *ent.Client
	APIKeyService           *APIKeyService
	WorkspaceSummaryService *UserWorkspaceSummaryService
	PricingService          *PricingService
}

type UserAPIKeyService struct {
	*AbstractService

	APIKeyService           *APIKeyService
	WorkspaceSummaryService *UserWorkspaceSummaryService
	PricingService          *PricingService
}

type UserAPIKey struct {
	ID                 string                          `json:"id"`
	ProjectID          string                          `json:"projectId"`
	Name               string                          `json:"name"`
	Type               apikey.Type                     `json:"type"`
	Status             apikey.Status                   `json:"status"`
	MaskedKey          string                          `json:"maskedKey"`
	ExpiresAt          *time.Time                      `json:"expiresAt,omitempty"`
	IPAllowlist        []string                        `json:"ipAllowlist"`
	AllowedModelIDs    []string                        `json:"allowedModelIds"`
	RequestLimit       *int64                          `json:"requestLimit,omitempty"`
	RequestLimitWindow UserAPIKeyLimitWindow           `json:"requestLimitWindow,omitempty"`
	CommercialLimits   *objects.APIKeyCommercialLimits `json:"commercialLimits,omitempty"`
	CreatedAt          time.Time                       `json:"createdAt"`
	UpdatedAt          time.Time                       `json:"updatedAt"`
}

type UserAPIKeySecretResult struct {
	APIKey UserAPIKey `json:"apiKey"`
	Secret string     `json:"secret"`
}

type UserAPIKeyModel struct {
	ModelID  string              `json:"modelId"`
	Currency string              `json:"currency,omitempty"`
	Price    *objects.ModelPrice `json:"price,omitempty"`
}

type UserAPIKeyCreateInput struct {
	ProjectID          int
	Name               string
	ExpiresAt          *time.Time
	IPAllowlist        []string
	AllowedModelIDs    []string
	RequestLimit       *int64
	RequestLimitWindow UserAPIKeyLimitWindow
	CommercialLimits   *objects.APIKeyCommercialLimits
}

type UserAPIKeyUpdateInput struct {
	Name                  *string
	Status                *apikey.Status
	ExpiresAt             *time.Time
	ClearExpiresAt        bool
	IPAllowlist           *[]string
	AllowedModelIDs       *[]string
	RequestLimit          *int64
	ClearRequestLimit     bool
	RequestLimitWindow    UserAPIKeyLimitWindow
	CommercialLimits      *objects.APIKeyCommercialLimits
	ClearCommercialLimits bool
}

func NewUserAPIKeyService(params UserAPIKeyServiceParams) *UserAPIKeyService {
	return &UserAPIKeyService{
		AbstractService:         &AbstractService{db: params.Ent},
		APIKeyService:           params.APIKeyService,
		WorkspaceSummaryService: params.WorkspaceSummaryService,
		PricingService:          params.PricingService,
	}
}

func (s *UserAPIKeyService) Models(ctx context.Context, viewer *ent.User, projectID int) ([]UserAPIKeyModel, error) {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, projectID, "user-api-key-models")
	if err != nil {
		return nil, err
	}
	projectRow, err := s.entFromContext(bypassCtx).Project.Get(bypassCtx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workspace for model catalog: %w", err)
	}
	available, err := s.WorkspaceSummaryService.availableModels(bypassCtx, projectRow)
	if err != nil {
		return nil, err
	}
	modelIDs := make([]string, 0, len(available))
	for modelID := range available {
		modelIDs = append(modelIDs, modelID)
	}
	sort.Strings(modelIDs)
	result := make([]UserAPIKeyModel, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		item := UserAPIKeyModel{ModelID: modelID}
		rule, err := s.PricingService.FindSellPrice(bypassCtx, projectID, modelID)
		if err == nil {
			price := rule.Price
			item.Price = &price
			item.Currency = rule.Currency
		} else if !errors.Is(err, ErrBillingPriceNotFound) {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *UserAPIKeyService) List(ctx context.Context, viewer *ent.User, projectID int) ([]UserAPIKey, error) {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, projectID, "user-api-key-list")
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
		return nil, fmt.Errorf("failed to list user api keys: %w", err)
	}

	result := make([]UserAPIKey, 0, len(rows))
	for _, row := range rows {
		result = append(result, projectUserAPIKey(row))
	}
	return result, nil
}

func (s *UserAPIKeyService) Create(ctx context.Context, viewer *ent.User, input UserAPIKeyCreateInput) (*UserAPIKeySecretResult, error) {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, input.ProjectID, "user-api-key-create")
	if err != nil {
		return nil, err
	}
	name, err := validateUserAPIKeyName(input.Name)
	if err != nil {
		return nil, err
	}
	allowlist, err := normalizeUserAPIKeyAllowlist(input.IPAllowlist)
	if err != nil {
		return nil, err
	}
	if err := validateUserAPIKeyExpiration(input.ExpiresAt, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := ValidateAPIKeyCommercialLimits(input.CommercialLimits); err != nil {
		return nil, err
	}
	profiles, err := userManagedProfiles(nil, &input.AllowedModelIDs, input.RequestLimit, false, input.RequestLimitWindow)
	if err != nil {
		return nil, err
	}
	secret, err := GenerateAPIKey(s.APIKeyService.keyPrefix)
	if err != nil {
		return nil, err
	}

	var created *ent.APIKey
	err = s.RunInTransaction(bypassCtx, func(txCtx context.Context) error {
		if err := s.APIKeyService.lockProjectForAPIKeyName(txCtx, input.ProjectID); err != nil {
			return err
		}
		client := s.entFromContext(txCtx)
		exists, err := client.APIKey.Query().Where(apikey.ProjectIDEQ(input.ProjectID), apikey.NameEQ(name)).Exist(txCtx)
		if err != nil {
			return fmt.Errorf("failed to check api key name: %w", err)
		}
		if exists {
			return xerrors.DuplicateNameError("API Key", name)
		}

		builder := client.APIKey.Create().
			SetUserID(viewer.ID).
			SetProjectID(input.ProjectID).
			SetName(name).
			SetKey(secret).
			SetType(apikey.TypePersonal).
			SetScopes([]string{"read_channels", "write_requests"}).
			SetIPAllowlist(allowlist).
			SetProfiles(profiles)
		if input.ExpiresAt != nil {
			builder.SetExpiresAt(input.ExpiresAt.UTC())
		}
		if input.CommercialLimits != nil {
			builder.SetCommercialLimits(input.CommercialLimits)
		}
		created, err = builder.Save(txCtx)
		if err != nil {
			return fmt.Errorf("failed to create user api key: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.APIKeyService.invalidateAPIKeyCaches(bypassCtx, secret)
	return &UserAPIKeySecretResult{APIKey: projectUserAPIKey(created), Secret: secret}, nil
}

func (s *UserAPIKeyService) Update(ctx context.Context, viewer *ent.User, projectID, id int, input UserAPIKeyUpdateInput) (*UserAPIKey, error) {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, projectID, "user-api-key-update")
	if err != nil {
		return nil, err
	}
	row, err := s.ownedKey(bypassCtx, viewer.ID, projectID, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		name, err := validateUserAPIKeyName(*input.Name)
		if err != nil {
			return nil, err
		}
		input.Name = &name
	}
	if input.Status != nil && *input.Status != apikey.StatusEnabled && *input.Status != apikey.StatusDisabled {
		return nil, fmt.Errorf("%w: status must be enabled or disabled", ErrUserAPIKeyInvalidInput)
	}
	if input.ClearExpiresAt {
		input.ExpiresAt = nil
	} else if err := validateUserAPIKeyExpiration(input.ExpiresAt, time.Now().UTC()); err != nil {
		return nil, err
	}
	if err := ValidateAPIKeyCommercialLimits(input.CommercialLimits); err != nil {
		return nil, err
	}

	var allowlist []string
	if input.IPAllowlist != nil {
		allowlist, err = normalizeUserAPIKeyAllowlist(*input.IPAllowlist)
		if err != nil {
			return nil, err
		}
	}
	profiles, err := userManagedProfiles(row.Profiles, input.AllowedModelIDs, input.RequestLimit, input.ClearRequestLimit, input.RequestLimitWindow)
	if err != nil {
		return nil, err
	}

	if input.Name != nil && *input.Name != row.Name {
		exists, err := s.entFromContext(bypassCtx).APIKey.Query().Where(
			apikey.ProjectIDEQ(projectID), apikey.NameEQ(*input.Name), apikey.IDNEQ(id),
		).Exist(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to check api key name: %w", err)
		}
		if exists {
			return nil, xerrors.DuplicateNameError("API Key", *input.Name)
		}
	}

	update := s.entFromContext(bypassCtx).APIKey.UpdateOneID(id).SetNillableName(input.Name)
	if input.Status != nil {
		update.SetStatus(*input.Status)
	}
	if input.ClearExpiresAt {
		update.ClearExpiresAt()
	} else if input.ExpiresAt != nil {
		update.SetExpiresAt(input.ExpiresAt.UTC())
	}
	if input.IPAllowlist != nil {
		update.SetIPAllowlist(allowlist)
	}
	if input.AllowedModelIDs != nil || input.RequestLimit != nil || input.ClearRequestLimit {
		update.SetProfiles(profiles)
	}
	if input.ClearCommercialLimits {
		update.ClearCommercialLimits()
	} else if input.CommercialLimits != nil {
		update.SetCommercialLimits(input.CommercialLimits)
	}
	updated, err := update.Save(bypassCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user api key: %w", err)
	}
	s.APIKeyService.invalidateAPIKeyCaches(bypassCtx, row.Key)
	result := projectUserAPIKey(updated)
	return &result, nil
}

func (s *UserAPIKeyService) Rotate(ctx context.Context, viewer *ent.User, projectID, id int) (*UserAPIKeySecretResult, error) {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, projectID, "user-api-key-rotate")
	if err != nil {
		return nil, err
	}
	row, err := s.ownedKey(bypassCtx, viewer.ID, projectID, id)
	if err != nil {
		return nil, err
	}
	secret, err := GenerateAPIKey(s.APIKeyService.keyPrefix)
	if err != nil {
		return nil, err
	}
	rotated, err := s.entFromContext(bypassCtx).APIKey.UpdateOneID(id).SetKey(secret).Save(bypassCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to rotate user api key: %w", err)
	}
	s.APIKeyService.invalidateAPIKeyCaches(bypassCtx, row.Key, secret)
	return &UserAPIKeySecretResult{APIKey: projectUserAPIKey(rotated), Secret: secret}, nil
}

func (s *UserAPIKeyService) Archive(ctx context.Context, viewer *ent.User, projectID, id int) error {
	bypassCtx, err := s.authorizedWorkspaceContext(ctx, viewer, projectID, "user-api-key-archive")
	if err != nil {
		return err
	}
	row, err := s.ownedKey(bypassCtx, viewer.ID, projectID, id)
	if err != nil {
		return err
	}
	if _, err := s.entFromContext(bypassCtx).APIKey.UpdateOneID(id).SetStatus(apikey.StatusArchived).Save(bypassCtx); err != nil {
		return fmt.Errorf("failed to archive user api key: %w", err)
	}
	s.APIKeyService.invalidateAPIKeyCaches(bypassCtx, row.Key)
	return nil
}

func (s *UserAPIKeyService) authorizedWorkspaceContext(ctx context.Context, viewer *ent.User, projectID int, reason string) (context.Context, error) {
	current, ok := contexts.GetUser(ctx)
	if !ok || current == nil || viewer == nil || current.ID != viewer.ID || projectID <= 0 {
		return nil, ErrUserAPIKeyDenied
	}
	bypassCtx := authz.WithSystemBypass(ctx, reason)
	exists, err := s.entFromContext(bypassCtx).UserProject.Query().Where(
		userproject.UserIDEQ(viewer.ID),
		userproject.ProjectIDEQ(projectID),
		userproject.HasProjectWith(project.StatusEQ(project.StatusActive)),
	).Exist(bypassCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to validate workspace membership: %w", err)
	}
	if !exists {
		return nil, ErrUserAPIKeyDenied
	}
	return bypassCtx, nil
}

func (s *UserAPIKeyService) ownedKey(ctx context.Context, userID, projectID, id int) (*ent.APIKey, error) {
	row, err := s.entFromContext(ctx).APIKey.Query().Where(
		apikey.IDEQ(id),
		apikey.UserIDEQ(userID),
		apikey.ProjectIDEQ(projectID),
		apikey.TypeIn(apikey.TypeUser, apikey.TypePersonal),
		apikey.StatusNEQ(apikey.StatusArchived),
	).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrUserAPIKeyDenied
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user api key: %w", err)
	}
	return row, nil
}

func projectUserAPIKey(row *ent.APIKey) UserAPIKey {
	models, requestLimit, window := userManagedProfileProjection(row.Profiles)
	return UserAPIKey{
		ID:                 workspaceObjectGUID(row.ID, ent.TypeAPIKey),
		ProjectID:          workspaceObjectGUID(row.ProjectID, ent.TypeProject),
		Name:               row.Name,
		Type:               row.Type,
		Status:             row.Status,
		MaskedKey:          maskAPIKeySecret(row.Key),
		ExpiresAt:          row.ExpiresAt,
		IPAllowlist:        cloneStringsOrEmpty(row.IPAllowlist),
		AllowedModelIDs:    models,
		RequestLimit:       requestLimit,
		RequestLimitWindow: window,
		CommercialLimits:   row.CommercialLimits,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func userManagedProfiles(
	existing *objects.APIKeyProfiles,
	modelIDs *[]string,
	requestLimit *int64,
	clearRequestLimit bool,
	window UserAPIKeyLimitWindow,
) (*objects.APIKeyProfiles, error) {
	profiles := &objects.APIKeyProfiles{}
	if existing != nil {
		profiles.ActiveProfile = existing.ActiveProfile
		profiles.Profiles = make([]objects.APIKeyProfile, len(existing.Profiles))
		for i := range existing.Profiles {
			profiles.Profiles[i] = *existing.Profiles[i].Clone()
		}
	}

	profileIndex := -1
	for i := range profiles.Profiles {
		if profiles.Profiles[i].Name == profiles.ActiveProfile {
			profileIndex = i
			break
		}
	}
	if profileIndex < 0 && (modelIDs != nil || requestLimit != nil) {
		profiles.ActiveProfile = "Personal"
		profiles.Profiles = append(profiles.Profiles, objects.APIKeyProfile{Name: "Personal", ModelMappings: []objects.ModelMapping{}})
		profileIndex = len(profiles.Profiles) - 1
	}
	if profileIndex < 0 {
		return profiles, nil
	}

	profile := &profiles.Profiles[profileIndex]
	if modelIDs != nil {
		profile.ModelIDs = uniqueTrimmedStrings(*modelIDs)
	}
	if clearRequestLimit {
		if profile.Quota != nil {
			profile.Quota.Requests = nil
			if profile.Quota.TotalTokens == nil && profile.Quota.Cost == nil {
				profile.Quota = nil
			}
		}
	}
	if requestLimit != nil {
		if *requestLimit <= 0 {
			return nil, fmt.Errorf("%w: request limit must be greater than zero", ErrUserAPIKeyInvalidInput)
		}
		period, err := userAPIKeyQuotaPeriod(window)
		if err != nil {
			return nil, err
		}
		if profile.Quota == nil {
			profile.Quota = &objects.APIKeyQuota{}
		}
		profile.Quota.Requests = requestLimit
		profile.Quota.Period = period
	}
	if err := validateProfileNames(profiles.Profiles); err != nil {
		return nil, err
	}
	if err := validateActiveProfile(profiles.ActiveProfile, profiles.Profiles); err != nil {
		return nil, err
	}
	if err := validateProfileFilters(profiles.Profiles); err != nil {
		return nil, err
	}
	if err := validateProfileQuota(profiles.Profiles); err != nil {
		return nil, err
	}
	return profiles, nil
}

func userManagedProfileProjection(profiles *objects.APIKeyProfiles) ([]string, *int64, UserAPIKeyLimitWindow) {
	if profiles == nil {
		return []string{}, nil, ""
	}
	for i := range profiles.Profiles {
		profile := &profiles.Profiles[i]
		if profile.Name != profiles.ActiveProfile {
			continue
		}
		var requests *int64
		var window UserAPIKeyLimitWindow
		if profile.Quota != nil {
			requests = profile.Quota.Requests
			window = userAPIKeyLimitWindow(profile.Quota.Period)
		}
		return cloneStringsOrEmpty(profile.ModelIDs), requests, window
	}
	return []string{}, nil, ""
}

func userAPIKeyQuotaPeriod(window UserAPIKeyLimitWindow) (objects.APIKeyQuotaPeriod, error) {
	switch window {
	case "", UserAPIKeyLimitWindowAllTime:
		return objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeAllTime}, nil
	case UserAPIKeyLimitWindowMinute:
		return objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypePastDuration, PastDuration: &objects.APIKeyQuotaPastDuration{Value: 1, Unit: objects.APIKeyQuotaPastDurationUnitMinute}}, nil
	case UserAPIKeyLimitWindowHour:
		return objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypePastDuration, PastDuration: &objects.APIKeyQuotaPastDuration{Value: 1, Unit: objects.APIKeyQuotaPastDurationUnitHour}}, nil
	case UserAPIKeyLimitWindowDay:
		return objects.APIKeyQuotaPeriod{Type: objects.APIKeyQuotaPeriodTypeCalendarDuration, CalendarDuration: &objects.APIKeyQuotaCalendarDuration{Unit: objects.APIKeyQuotaCalendarDurationUnitDay}}, nil
	default:
		return objects.APIKeyQuotaPeriod{}, fmt.Errorf("%w: unsupported request limit window", ErrUserAPIKeyInvalidInput)
	}
}

func userAPIKeyLimitWindow(period objects.APIKeyQuotaPeriod) UserAPIKeyLimitWindow {
	switch period.Type {
	case objects.APIKeyQuotaPeriodTypeAllTime:
		return UserAPIKeyLimitWindowAllTime
	case objects.APIKeyQuotaPeriodTypePastDuration:
		if period.PastDuration != nil {
			switch period.PastDuration.Unit {
			case objects.APIKeyQuotaPastDurationUnitMinute:
				return UserAPIKeyLimitWindowMinute
			case objects.APIKeyQuotaPastDurationUnitHour:
				return UserAPIKeyLimitWindowHour
			case objects.APIKeyQuotaPastDurationUnitDay:
				return UserAPIKeyLimitWindowDay
			}
		}
	case objects.APIKeyQuotaPeriodTypeCalendarDuration:
		return UserAPIKeyLimitWindowDay
	}
	return ""
}

func validateUserAPIKeyName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if utf8.RuneCountInString(name) == 0 || utf8.RuneCountInString(name) > 80 {
		return "", fmt.Errorf("%w: name must be between 1 and 80 characters", ErrUserAPIKeyInvalidInput)
	}
	return name, nil
}

func validateUserAPIKeyExpiration(expiresAt *time.Time, now time.Time) error {
	if expiresAt != nil && !expiresAt.After(now) {
		return fmt.Errorf("%w: expiration must be in the future", ErrUserAPIKeyInvalidInput)
	}
	return nil
}

func normalizeUserAPIKeyAllowlist(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr, addrErr := netip.ParseAddr(value)
			if addrErr != nil {
				return nil, fmt.Errorf("%w: invalid IP or CIDR %q", ErrUserAPIKeyInvalidInput, value)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		value = prefix.Masked().String()
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func ValidateAPIKeyRequestAccess(key *ent.APIKey, clientIP string, now time.Time) error {
	if key == nil {
		return ErrInvalidAPIKey
	}
	if key.ExpiresAt != nil && !key.ExpiresAt.After(now) {
		return fmt.Errorf("api key expired: %w", ErrInvalidAPIKey)
	}
	if len(key.IPAllowlist) == 0 {
		return nil
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(clientIP))
	if err != nil {
		return fmt.Errorf("api key client IP unavailable: %w", ErrInvalidAPIKey)
	}
	for _, allowed := range key.IPAllowlist {
		prefix, err := netip.ParsePrefix(allowed)
		if err == nil && prefix.Contains(addr) {
			return nil
		}
	}
	return fmt.Errorf("api key client IP denied: %w", ErrInvalidAPIKey)
}

func maskAPIKeySecret(secret string) string {
	if len(secret) <= 8 {
		return "********"
	}
	prefix := secret[:4]
	if before, _, ok := strings.Cut(secret, "-"); ok {
		prefix = before + "-"
	}
	return prefix + "..." + secret[len(secret)-4:]
}

func uniqueTrimmedStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cloneStringsOrEmpty(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return slices.Clone(values)
}

func workspaceObjectGUID(id int, objectType string) string {
	return fmt.Sprintf("gid://axonhub/%s/%d", objectType, id)
}
