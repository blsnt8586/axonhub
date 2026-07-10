package biz

import (
	"context"
	"fmt"
	"slices"
	"time"

	"entgo.io/ent/dialect/sql"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/project"
	"github.com/looplj/axonhub/internal/ent/request"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/ent/userproject"
	"github.com/looplj/axonhub/internal/ent/usersubscription"
)

type WorkspaceBlockReason string

const (
	WorkspaceBlockReasonNone                WorkspaceBlockReason = "none"
	WorkspaceBlockReasonUserInactive        WorkspaceBlockReason = "user_inactive"
	WorkspaceBlockReasonProjectMissing      WorkspaceBlockReason = "project_missing"
	WorkspaceBlockReasonAPIKeyMissing       WorkspaceBlockReason = "api_key_missing"
	WorkspaceBlockReasonModelUnavailable    WorkspaceBlockReason = "model_unavailable"
	WorkspaceBlockReasonAccountUnavailable  WorkspaceBlockReason = "account_unavailable"
	WorkspaceBlockReasonBalanceInsufficient WorkspaceBlockReason = "balance_insufficient"
)

type UserWorkspaceSummaryServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	SystemService         *SystemService `optional:"true"`
}

type UserWorkspaceSummaryService struct {
	ent                   *ent.Client
	billingAccountService *BillingAccountService
	system                *SystemService
}

func NewUserWorkspaceSummaryService(params UserWorkspaceSummaryServiceParams) *UserWorkspaceSummaryService {
	return &UserWorkspaceSummaryService{
		ent:                   params.Ent,
		billingAccountService: params.BillingAccountService,
		system:                params.SystemService,
	}
}

type UserWorkspaceSummary struct {
	User          UserWorkspaceSummaryUser          `json:"user"`
	Project       *UserWorkspaceSummaryProject      `json:"project,omitempty"`
	Billing       UserWorkspaceSummaryBilling       `json:"billing"`
	APIKeys       UserWorkspaceSummaryAPIKeys       `json:"apiKeys"`
	Models        UserWorkspaceSummaryModels        `json:"models"`
	Subscriptions UserWorkspaceSummarySubscriptions `json:"subscriptions"`
	Usage         UserWorkspaceSummaryUsage         `json:"usage"`
	Onboarding    UserWorkspaceSummaryOnboarding    `json:"onboarding"`
}

type UserWorkspaceSummaryUser struct {
	ID     int    `json:"id"`
	Email  string `json:"email"`
	Status string `json:"status"`
}

type UserWorkspaceSummaryProject struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	IsOwner bool   `json:"isOwner"`
}

type UserWorkspaceSummaryBilling struct {
	Currency          string `json:"currency"`
	BalanceMicros     int64  `json:"balanceMicros"`
	HeldBalanceMicros int64  `json:"heldBalanceMicros"`
	CreditLimitMicros int64  `json:"creditLimitMicros"`
	AvailableMicros   int64  `json:"availableMicros"`
	Status            string `json:"status"`
}

type UserWorkspaceSummaryAPIKeys struct {
	Total   int `json:"total"`
	Enabled int `json:"enabled"`
}

type UserWorkspaceSummaryModels struct {
	AvailableCount int `json:"availableCount"`
}

type UserWorkspaceSummarySubscriptions struct {
	ActiveCount    int  `json:"activeCount"`
	UsableCoverage bool `json:"usableCoverage"`
}

type UserWorkspaceSummaryUsage struct {
	RequestCount           int   `json:"requestCount"`
	TodayConsumptionMicros int64 `json:"todayConsumptionMicros"`
}

type UserWorkspaceSummaryOnboarding struct {
	CanUseAI    bool                 `json:"canUseAI"`
	BlockReason WorkspaceBlockReason `json:"blockReason"`
}

func (s *UserWorkspaceSummaryService) GetSummary(ctx context.Context, viewer *ent.User, selectedProjectID int, now time.Time) (*UserWorkspaceSummary, error) {
	if viewer == nil {
		return nil, ErrWorkspaceProjectDenied
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	return authz.RunWithSystemBypass(ctx, "user-workspace-summary", func(ctx context.Context) (*UserWorkspaceSummary, error) {
		currentUser, err := s.ent.User.Query().Where(user.IDEQ(viewer.ID)).Only(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query workspace user: %w", err)
		}

		account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(currentUser.ID))
		if err != nil {
			return nil, err
		}
		availableMicros := account.BalanceMicros + account.CreditLimitMicros - account.HeldBalanceMicros
		summary := &UserWorkspaceSummary{
			User: UserWorkspaceSummaryUser{
				ID:     currentUser.ID,
				Email:  currentUser.Email,
				Status: string(currentUser.Status),
			},
			Billing: UserWorkspaceSummaryBilling{
				Currency:          account.Currency,
				BalanceMicros:     account.BalanceMicros,
				HeldBalanceMicros: account.HeldBalanceMicros,
				CreditLimitMicros: account.CreditLimitMicros,
				AvailableMicros:   availableMicros,
				Status:            string(account.Status),
			},
		}

		projectRow, membership, err := s.resolveProject(ctx, currentUser.ID, selectedProjectID)
		if err != nil {
			return nil, err
		}
		if projectRow == nil {
			summary.Onboarding.BlockReason = WorkspaceBlockReasonProjectMissing
			return summary, nil
		}
		summary.Project = &UserWorkspaceSummaryProject{
			ID:      projectRow.ID,
			Name:    projectRow.Name,
			Status:  string(projectRow.Status),
			IsOwner: membership.IsOwner,
		}

		keys, err := s.ent.APIKey.Query().
			Where(apikey.UserIDEQ(currentUser.ID), apikey.ProjectIDEQ(projectRow.ID), apikey.StatusNEQ(apikey.StatusArchived)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to query workspace api keys: %w", err)
		}
		apiKeyIDs := make([]int, 0, len(keys))
		for _, key := range keys {
			apiKeyIDs = append(apiKeyIDs, key.ID)
			if key.Status == apikey.StatusEnabled {
				summary.APIKeys.Enabled++
			}
		}
		summary.APIKeys.Total = len(keys)

		availableModels, err := s.availableModels(ctx, projectRow)
		if err != nil {
			return nil, err
		}
		summary.Models.AvailableCount = len(availableModels)
		summary.Subscriptions.ActiveCount, summary.Subscriptions.UsableCoverage, err = s.activeSubscriptionStatus(ctx, currentUser.ID, projectRow.ID, availableModels, now)
		if err != nil {
			return nil, err
		}
		if len(apiKeyIDs) > 0 {
			summary.Usage.RequestCount, err = s.ent.Request.Query().
				Where(request.ProjectIDEQ(projectRow.ID), request.APIKeyIDIn(apiKeyIDs...)).
				Count(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to count workspace requests: %w", err)
			}
		}
		summary.Usage.TodayConsumptionMicros, err = s.todayConsumption(ctx, currentUser.ID, projectRow.ID, now)
		if err != nil {
			return nil, err
		}

		summary.Onboarding.BlockReason = workspaceBlockReason(currentUser, account, summary)
		summary.Onboarding.CanUseAI = summary.Onboarding.BlockReason == WorkspaceBlockReasonNone
		return summary, nil
	})
}

func (s *UserWorkspaceSummaryService) resolveProject(ctx context.Context, userID, selectedProjectID int) (*ent.Project, *ent.UserProject, error) {
	query := s.ent.Project.Query().
		Where(project.StatusEQ(project.StatusActive), project.HasUsersWith(user.IDEQ(userID))).
		Order(ent.Asc(project.FieldID))
	if selectedProjectID > 0 {
		query = query.Where(project.IDEQ(selectedProjectID))
	}

	projectRow, err := query.First(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			if selectedProjectID > 0 {
				return nil, nil, ErrWorkspaceProjectDenied
			}
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to resolve workspace project: %w", err)
	}
	membership, err := s.ent.UserProject.Query().
		Where(userproject.UserIDEQ(userID), userproject.ProjectIDEQ(projectRow.ID)).
		Only(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query workspace membership: %w", err)
	}
	return projectRow, membership, nil
}

func (s *UserWorkspaceSummaryService) availableModels(ctx context.Context, projectRow *ent.Project) (map[string]struct{}, error) {
	channels, err := s.ent.Channel.Query().Where(channel.StatusEQ(channel.StatusEnabled)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query available workspace models: %w", err)
	}
	if profile := projectRow.GetActiveProfile(); profile != nil {
		filtered := channels[:0]
		for _, row := range channels {
			if len(profile.ChannelIDs) > 0 && !slices.Contains(profile.ChannelIDs, row.ID) {
				continue
			}
			if !profile.MatchChannelTags(row.Tags) {
				continue
			}
			filtered = append(filtered, row)
		}
		channels = filtered
	}

	models := map[string]struct{}{}
	for _, row := range channels {
		for modelID := range (&Channel{Channel: row}).GetModelEntries() {
			models[modelID] = struct{}{}
		}
	}
	return models, nil
}

func (s *UserWorkspaceSummaryService) activeSubscriptionStatus(ctx context.Context, userID, projectID int, availableModels map[string]struct{}, now time.Time) (int, bool, error) {
	rows, err := s.ent.UserSubscription.Query().
		Where(
			usersubscription.UserIDEQ(userID),
			usersubscription.StatusEQ(usersubscription.StatusActive),
			usersubscription.StartsAtLTE(now),
			usersubscription.ExpiresAtGT(now),
		).
		All(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("failed to query workspace subscriptions: %w", err)
	}
	count := 0
	usableCoverage := false
	for _, row := range rows {
		if len(row.SupportedProjectIds) > 0 && !slices.Contains(row.SupportedProjectIds, projectID) {
			continue
		}
		if row.IncludedAmountMicros > 0 && row.UsedAmountMicros >= row.IncludedAmountMicros {
			continue
		}
		count++
		if len(row.SupportedModelIds) == 0 {
			usableCoverage = true
			continue
		}
		for _, modelID := range row.SupportedModelIds {
			if _, ok := availableModels[modelID]; ok {
				usableCoverage = true
				break
			}
		}
	}
	return count, usableCoverage, nil
}

func (s *UserWorkspaceSummaryService) todayConsumption(ctx context.Context, userID, projectID int, now time.Time) (int64, error) {
	loc := time.UTC
	if s.system != nil {
		loc = s.system.TimeLocation(ctx)
	}
	localNow := now.In(loc)
	start := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc).UTC()
	type row struct {
		Total int64 `json:"total"`
	}
	var rows []row
	err := s.ent.UsageBillingRecord.Query().
		Where(
			usagebillingrecord.UserIDEQ(userID),
			usagebillingrecord.ProjectIDEQ(projectID),
			usagebillingrecord.StatusEQ(usagebillingrecord.StatusCharged),
			usagebillingrecord.CreatedAtGTE(start),
		).
		Modify(func(selector *sql.Selector) {
			selector.Select(sql.As(fmt.Sprintf("COALESCE(SUM(%s), 0)", selector.C(usagebillingrecord.FieldChargeAmountMicros)), "total"))
		}).
		Scan(ctx, &rows)
	if err != nil {
		return 0, fmt.Errorf("failed to sum workspace consumption: %w", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return rows[0].Total, nil
}

func workspaceBlockReason(currentUser *ent.User, account *ent.BillingAccount, summary *UserWorkspaceSummary) WorkspaceBlockReason {
	if currentUser.Status != user.StatusActivated {
		return WorkspaceBlockReasonUserInactive
	}
	if summary.Project == nil {
		return WorkspaceBlockReasonProjectMissing
	}
	if summary.APIKeys.Enabled == 0 {
		return WorkspaceBlockReasonAPIKeyMissing
	}
	if summary.Models.AvailableCount == 0 {
		return WorkspaceBlockReasonModelUnavailable
	}
	if account.Status != billingaccount.StatusActive {
		return WorkspaceBlockReasonAccountUnavailable
	}
	if summary.Billing.AvailableMicros <= 0 && !summary.Subscriptions.UsableCoverage {
		return WorkspaceBlockReasonBalanceInsufficient
	}
	return WorkspaceBlockReasonNone
}
