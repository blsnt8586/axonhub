package biz

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/upstreamaccount"
	"github.com/looplj/axonhub/internal/ent/upstreamaccountpool"
	"github.com/looplj/axonhub/internal/objects"
)

type UpstreamAccountServiceParams struct {
	fx.In

	Ent *ent.Client
}

type UpstreamAccountService struct {
	*AbstractService
}

func NewUpstreamAccountService(params UpstreamAccountServiceParams) *UpstreamAccountService {
	return &UpstreamAccountService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

type UpstreamAccountCredentialsInput struct {
	APIKey  *string
	OAuth   *string
	RawJSON *string
	Headers []*objects.HeaderEntry
}

func (in *UpstreamAccountCredentialsInput) ToCredentials() objects.UpstreamAccountCredentials {
	if in == nil {
		return objects.UpstreamAccountCredentials{}
	}

	creds := objects.UpstreamAccountCredentials{}
	if in.APIKey != nil {
		creds.APIKey = strings.TrimSpace(*in.APIKey)
	}
	if in.OAuth != nil {
		creds.OAuth = strings.TrimSpace(*in.OAuth)
	}
	if in.RawJSON != nil {
		creds.RawJSON = strings.TrimSpace(*in.RawJSON)
	}
	if len(in.Headers) > 0 {
		creds.Headers = make([]objects.HeaderEntry, 0, len(in.Headers))
		for _, header := range in.Headers {
			if header == nil {
				continue
			}
			key := strings.TrimSpace(header.Key)
			if key == "" {
				continue
			}
			creds.Headers = append(creds.Headers, objects.HeaderEntry{
				Key:   key,
				Value: header.Value,
			})
		}
	}

	return creds
}

type UpstreamAccountProxyInput struct {
	Type     objects.ProxyType
	URL      *string
	Username *string
	Password *string
}

func (in *UpstreamAccountProxyInput) ToProxyConfig() *objects.ProxyConfig {
	if in == nil {
		return nil
	}

	proxy := &objects.ProxyConfig{
		Type: in.Type,
	}
	if in.URL != nil {
		proxy.URL = strings.TrimSpace(*in.URL)
	}
	if in.Username != nil {
		proxy.Username = strings.TrimSpace(*in.Username)
	}
	if in.Password != nil {
		proxy.Password = *in.Password
	}

	return proxy
}

type CreateUpstreamAccountPoolParams struct {
	ChannelID     int
	Name          string
	Status        upstreamaccountpool.Status
	Priority      int
	ModelPatterns []string
	ProjectIDs    []int
	Remark        *string
}

type UpdateUpstreamAccountPoolParams struct {
	Name          *string
	Status        *upstreamaccountpool.Status
	Priority      *int
	ModelPatterns []string
	ProjectIDs    []int
	Remark        *string
	ClearRemark   bool
}

type CreateUpstreamAccountParams struct {
	ChannelID        int
	PoolID           *int
	Name             string
	CredentialType   upstreamaccount.CredentialType
	Credentials      objects.UpstreamAccountCredentials
	Status           upstreamaccount.Status
	Schedulable      bool
	Priority         int
	Weight           int
	ConcurrencyLimit int
	ProxyConfig      *objects.ProxyConfig
	RateMultiplier   float64
	ExpiresAt        *time.Time
	QuotaLimitMicros int64
	QuotaUsedMicros  int64
	ErrorMessage     *string
	RateLimitResetAt *time.Time
	OverloadUntil    *time.Time
	CooldownUntil    *time.Time
	CooldownReason   *string
}

type UpdateUpstreamAccountParams struct {
	PoolID                *int
	ClearPool             bool
	Name                  *string
	CredentialType        *upstreamaccount.CredentialType
	Credentials           *objects.UpstreamAccountCredentials
	Status                *upstreamaccount.Status
	Schedulable           *bool
	Priority              *int
	Weight                *int
	ConcurrencyLimit      *int
	ProxyConfig           *objects.ProxyConfig
	ClearProxyConfig      bool
	RateMultiplier        *float64
	ExpiresAt             *time.Time
	ClearExpiresAt        bool
	QuotaLimitMicros      *int64
	QuotaUsedMicros       *int64
	ErrorMessage          *string
	ClearErrorMessage     bool
	RateLimitResetAt      *time.Time
	ClearRateLimitResetAt bool
	OverloadUntil         *time.Time
	ClearOverloadUntil    bool
	CooldownUntil         *time.Time
	ClearCooldownUntil    bool
	CooldownReason        *string
	ClearCooldownReason   bool
}

type UpstreamAccountTestResult struct {
	AccountID        int
	Success          bool
	Eligible         bool
	Message          string
	IneligibleReason *string
}

func (s *UpstreamAccountService) ListPools(ctx context.Context, channelID int, includeArchived bool) ([]*ent.UpstreamAccountPool, error) {
	query := s.entFromContext(ctx).UpstreamAccountPool.Query().
		Where(upstreamaccountpool.ChannelIDEQ(channelID)).
		Order(ent.Asc(upstreamaccountpool.FieldPriority), ent.Asc(upstreamaccountpool.FieldID))
	if !includeArchived {
		query.Where(upstreamaccountpool.StatusNEQ(upstreamaccountpool.StatusArchived))
	}

	return query.All(ctx)
}

func (s *UpstreamAccountService) ListAccounts(ctx context.Context, channelID int, includeArchived bool) ([]*ent.UpstreamAccount, error) {
	query := s.entFromContext(ctx).UpstreamAccount.Query().
		Where(upstreamaccount.ChannelIDEQ(channelID)).
		Order(ent.Asc(upstreamaccount.FieldPriority), ent.Desc(upstreamaccount.FieldWeight), ent.Asc(upstreamaccount.FieldID))
	if !includeArchived {
		query.Where(upstreamaccount.StatusNEQ(upstreamaccount.StatusArchived))
	}

	return query.All(ctx)
}

func (s *UpstreamAccountService) CreatePool(ctx context.Context, input CreateUpstreamAccountPoolParams) (*ent.UpstreamAccountPool, error) {
	if input.ChannelID <= 0 {
		return nil, fmt.Errorf("channel id is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("pool name is required")
	}
	status := input.Status
	if status == "" {
		status = upstreamaccountpool.StatusEnabled
	}

	create := s.entFromContext(ctx).UpstreamAccountPool.Create().
		SetChannelID(input.ChannelID).
		SetName(name).
		SetStatus(status).
		SetPriority(input.Priority).
		SetModelPatterns(cleanStringSlice(input.ModelPatterns)).
		SetProjectIds(cleanPositiveInts(input.ProjectIDs))
	if input.Remark != nil {
		create.SetRemark(strings.TrimSpace(*input.Remark))
	}

	return create.Save(ctx)
}

func (s *UpstreamAccountService) UpdatePool(ctx context.Context, id int, input UpdateUpstreamAccountPoolParams) (*ent.UpstreamAccountPool, error) {
	update := s.entFromContext(ctx).UpstreamAccountPool.UpdateOneID(id)
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("pool name cannot be empty")
		}
		update.SetName(name)
	}
	if input.Status != nil {
		update.SetStatus(*input.Status)
	}
	if input.Priority != nil {
		update.SetPriority(*input.Priority)
	}
	if input.ModelPatterns != nil {
		update.SetModelPatterns(cleanStringSlice(input.ModelPatterns))
	}
	if input.ProjectIDs != nil {
		update.SetProjectIds(cleanPositiveInts(input.ProjectIDs))
	}
	if input.ClearRemark {
		update.ClearRemark()
	} else if input.Remark != nil {
		update.SetRemark(strings.TrimSpace(*input.Remark))
	}

	return update.Save(ctx)
}

func (s *UpstreamAccountService) ArchivePool(ctx context.Context, id int) (*ent.UpstreamAccountPool, error) {
	return s.entFromContext(ctx).UpstreamAccountPool.UpdateOneID(id).
		SetStatus(upstreamaccountpool.StatusArchived).
		Save(ctx)
}

func (s *UpstreamAccountService) CreateAccount(ctx context.Context, input CreateUpstreamAccountParams) (*ent.UpstreamAccount, error) {
	if input.ChannelID <= 0 {
		return nil, fmt.Errorf("channel id is required")
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("account name is required")
	}
	if !input.Credentials.HasValue() {
		return nil, fmt.Errorf("account credentials are required")
	}
	credentialType := input.CredentialType
	if credentialType == "" {
		credentialType = upstreamaccount.CredentialTypeAPIKey
	}
	status := input.Status
	if status == "" {
		status = upstreamaccount.StatusActive
	}
	rateMultiplier := input.RateMultiplier
	if rateMultiplier <= 0 {
		rateMultiplier = 1
	}

	create := s.entFromContext(ctx).UpstreamAccount.Create().
		SetChannelID(input.ChannelID).
		SetNillablePoolID(input.PoolID).
		SetName(name).
		SetCredentialType(credentialType).
		SetCredentials(input.Credentials).
		SetStatus(status).
		SetSchedulable(input.Schedulable).
		SetPriority(input.Priority).
		SetWeight(normalizeWeight(input.Weight)).
		SetConcurrencyLimit(nonNegativeInt(input.ConcurrencyLimit)).
		SetRateMultiplier(rateMultiplier).
		SetNillableExpiresAt(input.ExpiresAt).
		SetQuotaLimitMicros(nonNegativeInt64(input.QuotaLimitMicros)).
		SetQuotaUsedMicros(nonNegativeInt64(input.QuotaUsedMicros)).
		SetNillableErrorMessage(cleanOptionalString(input.ErrorMessage)).
		SetNillableRateLimitResetAt(input.RateLimitResetAt).
		SetNillableOverloadUntil(input.OverloadUntil).
		SetNillableCooldownUntil(input.CooldownUntil).
		SetNillableCooldownReason(cleanOptionalString(input.CooldownReason))
	if input.ProxyConfig != nil {
		create.SetProxyConfig(input.ProxyConfig)
	}

	return create.Save(ctx)
}

func (s *UpstreamAccountService) UpdateAccount(ctx context.Context, id int, input UpdateUpstreamAccountParams) (*ent.UpstreamAccount, error) {
	update := s.entFromContext(ctx).UpstreamAccount.UpdateOneID(id)
	if input.ClearPool {
		update.ClearPoolID()
	} else if input.PoolID != nil {
		update.SetPoolID(*input.PoolID)
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return nil, fmt.Errorf("account name cannot be empty")
		}
		update.SetName(name)
	}
	if input.CredentialType != nil {
		update.SetCredentialType(*input.CredentialType)
	}
	if input.Credentials != nil && input.Credentials.HasValue() {
		update.SetCredentials(*input.Credentials)
	}
	if input.Status != nil {
		update.SetStatus(*input.Status)
	}
	if input.Schedulable != nil {
		update.SetSchedulable(*input.Schedulable)
	}
	if input.Priority != nil {
		update.SetPriority(*input.Priority)
	}
	if input.Weight != nil {
		update.SetWeight(normalizeWeight(*input.Weight))
	}
	if input.ConcurrencyLimit != nil {
		update.SetConcurrencyLimit(nonNegativeInt(*input.ConcurrencyLimit))
	}
	if input.ClearProxyConfig {
		update.ClearProxyConfig()
	} else if input.ProxyConfig != nil {
		update.SetProxyConfig(input.ProxyConfig)
	}
	if input.RateMultiplier != nil {
		if *input.RateMultiplier <= 0 {
			return nil, fmt.Errorf("rate multiplier must be greater than zero")
		}
		update.SetRateMultiplier(*input.RateMultiplier)
	}
	if input.ClearExpiresAt {
		update.ClearExpiresAt()
	} else if input.ExpiresAt != nil {
		update.SetExpiresAt(*input.ExpiresAt)
	}
	if input.QuotaLimitMicros != nil {
		update.SetQuotaLimitMicros(nonNegativeInt64(*input.QuotaLimitMicros))
	}
	if input.QuotaUsedMicros != nil {
		update.SetQuotaUsedMicros(nonNegativeInt64(*input.QuotaUsedMicros))
	}
	if input.ClearErrorMessage {
		update.ClearErrorMessage()
	} else if input.ErrorMessage != nil {
		update.SetErrorMessage(strings.TrimSpace(*input.ErrorMessage))
	}
	if input.ClearRateLimitResetAt {
		update.ClearRateLimitResetAt()
	} else if input.RateLimitResetAt != nil {
		update.SetRateLimitResetAt(*input.RateLimitResetAt)
	}
	if input.ClearOverloadUntil {
		update.ClearOverloadUntil()
	} else if input.OverloadUntil != nil {
		update.SetOverloadUntil(*input.OverloadUntil)
	}
	if input.ClearCooldownUntil {
		update.ClearCooldownUntil()
	} else if input.CooldownUntil != nil {
		update.SetCooldownUntil(*input.CooldownUntil)
	}
	if input.ClearCooldownReason {
		update.ClearCooldownReason()
	} else if input.CooldownReason != nil {
		update.SetCooldownReason(strings.TrimSpace(*input.CooldownReason))
	}

	return update.Save(ctx)
}

func (s *UpstreamAccountService) ArchiveAccount(ctx context.Context, id int) (*ent.UpstreamAccount, error) {
	return s.entFromContext(ctx).UpstreamAccount.UpdateOneID(id).
		SetStatus(upstreamaccount.StatusArchived).
		SetSchedulable(false).
		Save(ctx)
}

func (s *UpstreamAccountService) EligibleAccounts(ctx context.Context, channelID int, now time.Time) ([]*ent.UpstreamAccount, error) {
	accounts, err := s.ListAccounts(ctx, channelID, false)
	if err != nil {
		return nil, err
	}

	eligible := make([]*ent.UpstreamAccount, 0, len(accounts))
	for _, account := range accounts {
		if ok, _ := s.AccountEligibility(account, now); ok {
			eligible = append(eligible, account)
		}
	}

	return eligible, nil
}

func (s *UpstreamAccountService) AccountEligibility(account *ent.UpstreamAccount, now time.Time) (bool, string) {
	if account == nil {
		return false, "account is nil"
	}
	if account.Status != upstreamaccount.StatusActive {
		return false, "account is not active"
	}
	if !account.Schedulable {
		return false, "account is not schedulable"
	}
	if account.ExpiresAt != nil && !account.ExpiresAt.After(now) {
		return false, "account is expired"
	}
	if account.QuotaLimitMicros > 0 && account.QuotaUsedMicros >= account.QuotaLimitMicros {
		return false, "quota exhausted"
	}
	if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
		return false, "rate limited"
	}
	if account.OverloadUntil != nil && account.OverloadUntil.After(now) {
		return false, "overloaded"
	}
	if account.CooldownUntil != nil && account.CooldownUntil.After(now) {
		if account.CooldownReason != nil && strings.TrimSpace(*account.CooldownReason) != "" {
			return false, "cooling down: " + strings.TrimSpace(*account.CooldownReason)
		}
		return false, "cooling down"
	}

	return true, ""
}

func (s *UpstreamAccountService) ChannelUsesCredentialFallback(ctx context.Context, channelID int) (bool, error) {
	count, err := s.entFromContext(ctx).UpstreamAccountPool.Query().
		Where(
			upstreamaccountpool.ChannelIDEQ(channelID),
			upstreamaccountpool.StatusNEQ(upstreamaccountpool.StatusArchived),
		).
		Count(ctx)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func (s *UpstreamAccountService) TestAccount(ctx context.Context, id int) (*UpstreamAccountTestResult, error) {
	account, err := s.entFromContext(ctx).UpstreamAccount.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	eligible, reason := s.AccountEligibility(account, time.Now())
	if eligible {
		return &UpstreamAccountTestResult{
			AccountID: id,
			Success:   true,
			Eligible:  true,
			Message:   "account is eligible for scheduling",
		}, nil
	}

	return &UpstreamAccountTestResult{
		AccountID:        id,
		Success:          false,
		Eligible:         false,
		Message:          "account is not eligible for scheduling",
		IneligibleReason: &reason,
	}, nil
}

func cleanStringSlice(values []string) []string {
	cleaned := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		cleaned = append(cleaned, trimmed)
	}
	return cleaned
}

func cleanPositiveInts(values []int) []int {
	cleaned := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}

func cleanOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	cleaned := strings.TrimSpace(*value)
	return &cleaned
}

func normalizeWeight(value int) int {
	if value <= 0 {
		return 1
	}
	return value
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeInt64(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}
