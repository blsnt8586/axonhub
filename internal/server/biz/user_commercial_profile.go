package biz

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	entclient "github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/predicate"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/user"
)

const (
	userCommercialProfileDefaultLimit = 10
	userCommercialProfileMaxLimit     = 100
)

type UserCommercialProfileServiceParams struct {
	fx.In

	Ent                   *entclient.Client
	SystemService         *SystemService
	BillingAccountService *BillingAccountService
}

type UserCommercialProfileService struct {
	ent                   *entclient.Client
	system                *SystemService
	billingAccountService *BillingAccountService
}

func NewUserCommercialProfileService(params UserCommercialProfileServiceParams) *UserCommercialProfileService {
	return &UserCommercialProfileService{
		ent:                   params.Ent,
		system:                params.SystemService,
		billingAccountService: params.BillingAccountService,
	}
}

type UserCommercialProfileFilter struct {
	From        *time.Time
	To          *time.Time
	ProjectID   *int
	APIKeyID    *int
	ModelID     string
	RequestType string
	Limit       int
}

type UserCommercialProfile struct {
	User                  UserCommercialProfileUser             `json:"user"`
	Filter                UserCommercialProfileFilterPayload    `json:"filter"`
	BillingAccount        UserCommercialProfileBillingAccount   `json:"billingAccount"`
	Totals                UserCommercialProfileTotals           `json:"totals"`
	TopModels             []UserCommercialProfileRankedItem     `json:"topModels"`
	TopProjects           []UserCommercialProfileRankedItem     `json:"topProjects"`
	TopAPIKeys            []UserCommercialProfileRankedItem     `json:"topApiKeys"`
	RecentCharges         []UserCommercialProfileUsageCharge    `json:"recentCharges"`
	RecentRequests        []UserCommercialProfileRequest        `json:"recentRequests"`
	RecentBillingFailures []UserCommercialProfileBillingFailure `json:"recentBillingFailures"`
}

type UserCommercialProfileUser struct {
	ID    int    `json:"id"`
	Email string `json:"email"`
}

type UserCommercialProfileFilterPayload struct {
	From        *time.Time `json:"from,omitempty"`
	To          *time.Time `json:"to,omitempty"`
	ProjectID   *int       `json:"projectId,omitempty"`
	APIKeyID    *int       `json:"apiKeyId,omitempty"`
	ModelID     string     `json:"modelId,omitempty"`
	RequestType string     `json:"requestType,omitempty"`
	Limit       int        `json:"limit"`
}

type UserCommercialProfileBillingAccount struct {
	ID                  int    `json:"id"`
	OwnerType           string `json:"ownerType"`
	OwnerID             int    `json:"ownerId"`
	Currency            string `json:"currency"`
	BalanceMicros       int64  `json:"balanceMicros"`
	HeldBalanceMicros   int64  `json:"heldBalanceMicros"`
	CreditLimitMicros   int64  `json:"creditLimitMicros"`
	AvailableMicros     int64  `json:"availableMicros"`
	Status              string `json:"status"`
	TotalRechargeMicros int64  `json:"totalRechargeMicros"`
}

type UserCommercialProfileTotals struct {
	TotalRechargeMicros    int64 `json:"totalRechargeMicros"`
	TotalConsumptionMicros int64 `json:"totalConsumptionMicros"`
	TodayConsumptionMicros int64 `json:"todayConsumptionMicros"`
	MonthConsumptionMicros int64 `json:"monthConsumptionMicros"`
	RequestCount           int   `json:"requestCount"`
	FailureCount           int   `json:"failureCount"`
}

type UserCommercialProfileRankedItem struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	ChargeAmountMicros int64  `json:"chargeAmountMicros"`
	RequestCount       int    `json:"requestCount"`
}

type UserCommercialProfileUsageCharge struct {
	ID                 int       `json:"id"`
	CreatedAt          time.Time `json:"createdAt"`
	UsageLogID         int       `json:"usageLogId"`
	BillingAccountID   int       `json:"billingAccountId"`
	ProjectID          int       `json:"projectId"`
	ProjectName        string    `json:"projectName"`
	UserID             *int      `json:"userId,omitempty"`
	APIKeyID           *int      `json:"apiKeyId,omitempty"`
	APIKeyName         string    `json:"apiKeyName,omitempty"`
	ModelID            string    `json:"modelId"`
	RequestType        string    `json:"requestType"`
	ChargeAmountMicros int64     `json:"chargeAmountMicros"`
	CostAmountMicros   int64     `json:"costAmountMicros"`
	Currency           string    `json:"currency"`
	Status             string    `json:"status"`
	Error              string    `json:"error"`
}

type UserCommercialProfileRequest struct {
	ID           int       `json:"id"`
	CreatedAt    time.Time `json:"createdAt"`
	ProjectID    int       `json:"projectId"`
	ProjectName  string    `json:"projectName"`
	APIKeyID     *int      `json:"apiKeyId,omitempty"`
	APIKeyName   string    `json:"apiKeyName,omitempty"`
	ModelID      string    `json:"modelId"`
	RequestType  string    `json:"requestType"`
	Format       string    `json:"format"`
	Source       string    `json:"source"`
	Status       string    `json:"status"`
	Stream       bool      `json:"stream"`
	LatencyMs    *int64    `json:"latencyMs,omitempty"`
	FirstTokenMs *int64    `json:"firstTokenMs,omitempty"`
	ReasoningMs  *int64    `json:"reasoningMs,omitempty"`
}

type UserCommercialProfileBillingFailure struct {
	ID                 int       `json:"id"`
	CreatedAt          time.Time `json:"createdAt"`
	UsageLogID         int       `json:"usageLogId"`
	ProjectID          int       `json:"projectId"`
	ProjectName        string    `json:"projectName"`
	APIKeyID           *int      `json:"apiKeyId,omitempty"`
	APIKeyName         string    `json:"apiKeyName,omitempty"`
	ModelID            string    `json:"modelId"`
	RequestType        string    `json:"requestType"`
	ChargeAmountMicros int64     `json:"chargeAmountMicros"`
	Currency           string    `json:"currency"`
	Status             string    `json:"status"`
	Error              string    `json:"error"`
}

func (s *UserCommercialProfileService) GetProfile(ctx context.Context, viewer *entclient.User, targetUserID int, filter UserCommercialProfileFilter) (*UserCommercialProfile, error) {
	if viewer == nil {
		return nil, ErrCommercialProfileDenied
	}
	if targetUserID <= 0 {
		targetUserID = viewer.ID
	}
	if !viewer.IsOwner && viewer.ID != targetUserID {
		return nil, ErrCommercialProfileDenied
	}

	normalized, err := normalizeCommercialProfileFilter(filter)
	if err != nil {
		return nil, err
	}

	return authz.RunWithSystemBypass(ctx, "user-commercial-profile", func(ctx context.Context) (*UserCommercialProfile, error) {
		target, err := s.ent.User.Query().
			Where(user.IDEQ(targetUserID)).
			Only(ctx)
		if err != nil {
			if entclient.IsNotFound(err) {
				return nil, fmt.Errorf("user not found: %d", targetUserID)
			}
			return nil, fmt.Errorf("failed to query user: %w", err)
		}

		account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(target.ID))
		if err != nil {
			return nil, err
		}

		apiKeys, err := s.ent.APIKey.Query().
			Where(apikey.UserIDEQ(target.ID)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query api keys: %w", err)
		}
		apiKeyIDs, apiKeyNames := commercialProfileAPIKeyMaps(apiKeys)

		usageRows, err := s.queryUsageRows(ctx, target.ID, account.ID, normalized)
		if err != nil {
			return nil, err
		}
		projectNames, err := s.projectNames(ctx, usageProjectIDs(usageRows))
		if err != nil {
			return nil, err
		}

		requestRows, err := s.queryRequestRows(ctx, apiKeyIDs, normalized)
		if err != nil {
			return nil, err
		}
		requestProjectNames, err := s.projectNames(ctx, requestProjectIDs(requestRows))
		if err != nil {
			return nil, err
		}
		for id, name := range requestProjectNames {
			projectNames[id] = name
		}

		totalRecharge, err := s.totalRecharge(ctx, account.ID)
		if err != nil {
			return nil, err
		}

		todayStart, monthStart := s.currentCommercialWindows(ctx, time.Now().UTC())
		profile := &UserCommercialProfile{
			User: UserCommercialProfileUser{
				ID:    target.ID,
				Email: target.Email,
			},
			Filter: UserCommercialProfileFilterPayload{
				From:        normalized.From,
				To:          normalized.To,
				ProjectID:   normalized.ProjectID,
				APIKeyID:    normalized.APIKeyID,
				ModelID:     normalized.ModelID,
				RequestType: normalized.RequestType,
				Limit:       normalized.Limit,
			},
			BillingAccount: UserCommercialProfileBillingAccount{
				ID:                  account.ID,
				OwnerType:           string(account.OwnerType),
				OwnerID:             account.OwnerID,
				Currency:            account.Currency,
				BalanceMicros:       account.BalanceMicros,
				HeldBalanceMicros:   account.HeldBalanceMicros,
				CreditLimitMicros:   account.CreditLimitMicros,
				AvailableMicros:     account.BalanceMicros + account.CreditLimitMicros - account.HeldBalanceMicros,
				Status:              string(account.Status),
				TotalRechargeMicros: totalRecharge,
			},
			Totals: UserCommercialProfileTotals{
				TotalRechargeMicros: totalRecharge,
				RequestCount:        len(requestRows),
			},
		}

		modelRanks := map[string]*UserCommercialProfileRankedItem{}
		projectRanks := map[string]*UserCommercialProfileRankedItem{}
		apiKeyRanks := map[string]*UserCommercialProfileRankedItem{}
		for _, row := range usageRows {
			if row.Status == usagebillingrecord.StatusFailed {
				profile.Totals.FailureCount++
			}
			if row.Status != usagebillingrecord.StatusCharged {
				continue
			}
			profile.Totals.TotalConsumptionMicros += row.ChargeAmountMicros
			if !row.CreatedAt.Before(todayStart) {
				profile.Totals.TodayConsumptionMicros += row.ChargeAmountMicros
			}
			if !row.CreatedAt.Before(monthStart) {
				profile.Totals.MonthConsumptionMicros += row.ChargeAmountMicros
			}
			addCommercialRank(modelRanks, row.ModelID, row.ModelID, row.ChargeAmountMicros)
			projectName := projectNames[row.ProjectID]
			if projectName == "" {
				projectName = fmt.Sprintf("Project %d", row.ProjectID)
			}
			addCommercialRank(projectRanks, fmt.Sprint(row.ProjectID), projectName, row.ChargeAmountMicros)
			if row.APIKeyID > 0 {
				apiKeyName := apiKeyNames[row.APIKeyID]
				if apiKeyName == "" {
					apiKeyName = fmt.Sprintf("API Key %d", row.APIKeyID)
				}
				addCommercialRank(apiKeyRanks, fmt.Sprint(row.APIKeyID), apiKeyName, row.ChargeAmountMicros)
			}
		}
		for _, row := range requestRows {
			if row.Status == request.StatusFailed {
				profile.Totals.FailureCount++
			}
		}

		profile.TopModels = sortedCommercialRanks(modelRanks, normalized.Limit)
		profile.TopProjects = sortedCommercialRanks(projectRanks, normalized.Limit)
		profile.TopAPIKeys = sortedCommercialRanks(apiKeyRanks, normalized.Limit)
		profile.RecentCharges = recentUsageCharges(usageRows, projectNames, apiKeyNames, normalized.Limit)
		profile.RecentRequests = recentRequests(requestRows, projectNames, apiKeyNames, normalized.Limit)
		profile.RecentBillingFailures = recentBillingFailures(usageRows, projectNames, apiKeyNames, normalized.Limit)

		return profile, nil
	})
}

func normalizeCommercialProfileFilter(filter UserCommercialProfileFilter) (UserCommercialProfileFilter, error) {
	filter.ModelID = strings.TrimSpace(filter.ModelID)
	filter.RequestType = strings.TrimSpace(filter.RequestType)
	if filter.Limit <= 0 {
		filter.Limit = userCommercialProfileDefaultLimit
	}
	if filter.Limit > userCommercialProfileMaxLimit {
		filter.Limit = userCommercialProfileMaxLimit
	}
	if filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return filter, fmt.Errorf("from must be before to")
	}
	if filter.ProjectID != nil && *filter.ProjectID <= 0 {
		return filter, fmt.Errorf("project id must be positive")
	}
	if filter.APIKeyID != nil && *filter.APIKeyID <= 0 {
		return filter, fmt.Errorf("api key id must be positive")
	}
	if filter.RequestType != "" && !validCommercialProfileRequestType(filter.RequestType) {
		return filter, fmt.Errorf("unsupported request type %q", filter.RequestType)
	}

	return filter, nil
}

func validCommercialProfileRequestType(value string) bool {
	switch usagebillingrecord.RequestType(value) {
	case usagebillingrecord.RequestTypeChat,
		usagebillingrecord.RequestTypeImage,
		usagebillingrecord.RequestTypeVideo,
		usagebillingrecord.RequestTypeEmbedding,
		usagebillingrecord.RequestTypeAudio,
		usagebillingrecord.RequestTypeOther:
		return true
	default:
		return false
	}
}

func (s *UserCommercialProfileService) queryUsageRows(ctx context.Context, userID, accountID int, filter UserCommercialProfileFilter) ([]*entclient.UsageBillingRecord, error) {
	query := s.ent.UsageBillingRecord.Query().
		Where(usagebillingrecord.BillingAccountIDEQ(accountID))
	if filter.From != nil {
		query.Where(usagebillingrecord.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		query.Where(usagebillingrecord.CreatedAtLTE(*filter.To))
	}
	if filter.ProjectID != nil {
		query.Where(usagebillingrecord.ProjectIDEQ(*filter.ProjectID))
	}
	if filter.APIKeyID != nil {
		query.Where(usagebillingrecord.APIKeyIDEQ(*filter.APIKeyID))
	}
	if filter.ModelID != "" {
		query.Where(usagebillingrecord.ModelIDEQ(filter.ModelID))
	}
	if filter.RequestType != "" {
		query.Where(usagebillingrecord.RequestTypeEQ(usagebillingrecord.RequestType(filter.RequestType)))
	}

	rows, err := query.Order(entclient.Desc(usagebillingrecord.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query usage billing records for user %d: %w", userID, err)
	}

	return rows, nil
}

func (s *UserCommercialProfileService) queryRequestRows(ctx context.Context, apiKeyIDs []int, filter UserCommercialProfileFilter) ([]*entclient.Request, error) {
	if len(apiKeyIDs) == 0 {
		return []*entclient.Request{}, nil
	}

	query := s.ent.Request.Query().
		Where(request.APIKeyIDIn(apiKeyIDs...))
	if filter.From != nil {
		query.Where(request.CreatedAtGTE(*filter.From))
	}
	if filter.To != nil {
		query.Where(request.CreatedAtLTE(*filter.To))
	}
	if filter.ProjectID != nil {
		query.Where(request.ProjectIDEQ(*filter.ProjectID))
	}
	if filter.APIKeyID != nil {
		query.Where(request.APIKeyIDEQ(*filter.APIKeyID))
	}
	if filter.ModelID != "" {
		query.Where(request.ModelIDEQ(filter.ModelID))
	}
	if filter.RequestType != "" {
		query.Where(requestTypePredicate(filter.RequestType))
	}

	rows, err := query.Order(entclient.Desc(request.FieldCreatedAt)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query requests: %w", err)
	}

	return rows, nil
}

func requestTypePredicate(requestType string) predicate.Request {
	switch usagebillingrecord.RequestType(requestType) {
	case usagebillingrecord.RequestTypeImage:
		return request.FormatContains("image")
	case usagebillingrecord.RequestTypeVideo:
		return request.FormatContains("video")
	case usagebillingrecord.RequestTypeEmbedding:
		return request.FormatContains("embedding")
	case usagebillingrecord.RequestTypeAudio:
		return request.Or(request.FormatContains("audio"), request.FormatContains("speech"), request.FormatContains("transcription"))
	case usagebillingrecord.RequestTypeOther:
		return request.Not(request.Or(
			request.FormatContains("chat"),
			request.FormatContains("messages"),
			request.FormatContains("responses"),
			request.FormatContains("image"),
			request.FormatContains("video"),
			request.FormatContains("embedding"),
			request.FormatContains("audio"),
			request.FormatContains("speech"),
			request.FormatContains("transcription"),
		))
	default:
		return request.Or(request.FormatContains("chat"), request.FormatContains("messages"), request.FormatContains("responses"))
	}
}

func (s *UserCommercialProfileService) totalRecharge(ctx context.Context, billingAccountID int) (int64, error) {
	rows, err := s.ent.LedgerTransaction.Query().
		Where(
			ledgertransaction.BillingAccountIDEQ(billingAccountID),
			ledgertransaction.DirectionEQ(ledgertransaction.DirectionCredit),
			ledgertransaction.StatusEQ(ledgertransaction.StatusPosted),
			ledgertransaction.TypeIn(
				ledgertransaction.TypePaymentRecharge,
				ledgertransaction.TypeAdminAdjustment,
				ledgertransaction.TypeSubscriptionGrant,
				ledgertransaction.TypeRedeemCode,
				ledgertransaction.TypeAffiliateRebate,
			),
		).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query recharge ledger transactions: %w", err)
	}

	var total int64
	for _, row := range rows {
		total += row.AmountMicros
	}

	return total, nil
}

func (s *UserCommercialProfileService) currentCommercialWindows(ctx context.Context, now time.Time) (time.Time, time.Time) {
	loc := time.UTC
	if s.system != nil {
		loc = s.system.TimeLocation(ctx)
	}
	localNow := now.In(loc)
	todayStart := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc).UTC()
	monthStart := time.Date(localNow.Year(), localNow.Month(), 1, 0, 0, 0, 0, loc).UTC()

	return todayStart, monthStart
}

func (s *UserCommercialProfileService) projectNames(ctx context.Context, ids []int) (map[int]string, error) {
	names := map[int]string{}
	if len(ids) == 0 {
		return names, nil
	}

	rows, err := s.ent.Project.Query().
		Where(project.IDIn(uniqueInts(ids)...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query project names: %w", err)
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}

	return names, nil
}

func commercialProfileAPIKeyMaps(rows []*entclient.APIKey) ([]int, map[int]string) {
	ids := make([]int, 0, len(rows))
	names := make(map[int]string, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		names[row.ID] = row.Name
	}

	return ids, names
}

func usageProjectIDs(rows []*entclient.UsageBillingRecord) []int {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProjectID)
	}

	return uniqueInts(ids)
}

func requestProjectIDs(rows []*entclient.Request) []int {
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ProjectID)
	}

	return uniqueInts(ids)
}

func uniqueInts(values []int) []int {
	seen := make(map[int]struct{}, len(values))
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}

func addCommercialRank(rows map[string]*UserCommercialProfileRankedItem, id, name string, amount int64) {
	row := rows[id]
	if row == nil {
		row = &UserCommercialProfileRankedItem{ID: id, Name: name}
		rows[id] = row
	}
	row.ChargeAmountMicros += amount
	row.RequestCount++
}

func sortedCommercialRanks(rows map[string]*UserCommercialProfileRankedItem, limit int) []UserCommercialProfileRankedItem {
	result := make([]UserCommercialProfileRankedItem, 0, len(rows))
	for _, row := range rows {
		result = append(result, *row)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].ChargeAmountMicros == result[j].ChargeAmountMicros {
			return result[i].Name < result[j].Name
		}
		return result[i].ChargeAmountMicros > result[j].ChargeAmountMicros
	})
	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

func recentUsageCharges(rows []*entclient.UsageBillingRecord, projectNames map[int]string, apiKeyNames map[int]string, limit int) []UserCommercialProfileUsageCharge {
	result := make([]UserCommercialProfileUsageCharge, 0, min(len(rows), limit))
	for _, row := range rows {
		if len(result) >= limit {
			break
		}
		result = append(result, usageChargePayload(row, projectNames, apiKeyNames))
	}

	return result
}

func recentBillingFailures(rows []*entclient.UsageBillingRecord, projectNames map[int]string, apiKeyNames map[int]string, limit int) []UserCommercialProfileBillingFailure {
	result := make([]UserCommercialProfileBillingFailure, 0, min(len(rows), limit))
	for _, row := range rows {
		if row.Status != usagebillingrecord.StatusFailed {
			continue
		}
		if len(result) >= limit {
			break
		}
		payload := usageChargePayload(row, projectNames, apiKeyNames)
		result = append(result, UserCommercialProfileBillingFailure{
			ID:                 payload.ID,
			CreatedAt:          payload.CreatedAt,
			UsageLogID:         payload.UsageLogID,
			ProjectID:          payload.ProjectID,
			ProjectName:        payload.ProjectName,
			APIKeyID:           payload.APIKeyID,
			APIKeyName:         payload.APIKeyName,
			ModelID:            payload.ModelID,
			RequestType:        payload.RequestType,
			ChargeAmountMicros: payload.ChargeAmountMicros,
			Currency:           payload.Currency,
			Status:             payload.Status,
			Error:              payload.Error,
		})
	}

	return result
}

func usageChargePayload(row *entclient.UsageBillingRecord, projectNames map[int]string, apiKeyNames map[int]string) UserCommercialProfileUsageCharge {
	projectName := projectNames[row.ProjectID]
	if projectName == "" {
		projectName = fmt.Sprintf("Project %d", row.ProjectID)
	}
	var userID *int
	if row.UserID > 0 {
		userID = &row.UserID
	}
	var apiKeyID *int
	apiKeyName := ""
	if row.APIKeyID > 0 {
		apiKeyID = &row.APIKeyID
		apiKeyName = apiKeyNames[row.APIKeyID]
	}

	return UserCommercialProfileUsageCharge{
		ID:                 row.ID,
		CreatedAt:          row.CreatedAt,
		UsageLogID:         row.UsageLogID,
		BillingAccountID:   row.BillingAccountID,
		ProjectID:          row.ProjectID,
		ProjectName:        projectName,
		UserID:             userID,
		APIKeyID:           apiKeyID,
		APIKeyName:         apiKeyName,
		ModelID:            row.ModelID,
		RequestType:        string(row.RequestType),
		ChargeAmountMicros: row.ChargeAmountMicros,
		CostAmountMicros:   row.CostAmountMicros,
		Currency:           row.Currency,
		Status:             string(row.Status),
		Error:              row.Error,
	}
}

func recentRequests(rows []*entclient.Request, projectNames map[int]string, apiKeyNames map[int]string, limit int) []UserCommercialProfileRequest {
	result := make([]UserCommercialProfileRequest, 0, min(len(rows), limit))
	for _, row := range rows {
		if len(result) >= limit {
			break
		}
		projectName := projectNames[row.ProjectID]
		if projectName == "" {
			projectName = fmt.Sprintf("Project %d", row.ProjectID)
		}
		var apiKeyID *int
		apiKeyName := ""
		if row.APIKeyID > 0 {
			apiKeyID = &row.APIKeyID
			apiKeyName = apiKeyNames[row.APIKeyID]
		}
		result = append(result, UserCommercialProfileRequest{
			ID:           row.ID,
			CreatedAt:    row.CreatedAt,
			ProjectID:    row.ProjectID,
			ProjectName:  projectName,
			APIKeyID:     apiKeyID,
			APIKeyName:   apiKeyName,
			ModelID:      row.ModelID,
			RequestType:  inferCommercialRequestType(row.Format),
			Format:       row.Format,
			Source:       string(row.Source),
			Status:       string(row.Status),
			Stream:       row.Stream,
			LatencyMs:    row.MetricsLatencyMs,
			FirstTokenMs: row.MetricsFirstTokenLatencyMs,
			ReasoningMs:  row.MetricsReasoningDurationMs,
		})
	}

	return result
}
