package biz

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
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

	mu      sync.Mutex
	runtime map[int]*upstreamAccountRuntime
}

func NewUpstreamAccountService(params UpstreamAccountServiceParams) *UpstreamAccountService {
	return &UpstreamAccountService{
		AbstractService: &AbstractService{db: params.Ent},
		runtime:         make(map[int]*upstreamAccountRuntime),
	}
}

const (
	defaultAccountRateLimitCooldown = time.Minute
	defaultAccountOverloadCooldown  = 30 * time.Second
	defaultAccountNetworkCooldown   = 30 * time.Second
)

var ErrUpstreamAccountPoolExhausted = errors.New("upstream account pool exhausted")

type UpstreamAccountPoolExhaustedError struct {
	ChannelID int
	ModelID   string
	Reason    string
}

func (e *UpstreamAccountPoolExhaustedError) Error() string {
	reason := strings.TrimSpace(e.Reason)
	if reason == "" {
		reason = "no eligible upstream account"
	}

	if e.ModelID == "" {
		return fmt.Sprintf("%s for channel %d: %s", ErrUpstreamAccountPoolExhausted, e.ChannelID, reason)
	}

	return fmt.Sprintf("%s for channel %d model %s: %s", ErrUpstreamAccountPoolExhausted, e.ChannelID, e.ModelID, reason)
}

func (e *UpstreamAccountPoolExhaustedError) Unwrap() error {
	return ErrUpstreamAccountPoolExhausted
}

type UpstreamAccountSelectionInput struct {
	ChannelID         int
	ModelID           string
	ProjectID         int
	ExcludeAccountIDs map[int]struct{}
	Now               time.Time
}

type UpstreamAccountFailureInput struct {
	AccountID  int
	StatusCode int
	Message    string
	Now        time.Time
}

type upstreamAccountRuntime struct {
	inFlight    int
	successes   int
	failures    int
	latencyEWMA float64
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

func (s *UpstreamAccountService) SelectAccountForRequest(ctx context.Context, input UpstreamAccountSelectionInput) (*ent.UpstreamAccount, func(), error) {
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}

	pools, err := s.entFromContext(ctx).UpstreamAccountPool.Query().
		Where(
			upstreamaccountpool.ChannelIDEQ(input.ChannelID),
			upstreamaccountpool.StatusEQ(upstreamaccountpool.StatusEnabled),
		).
		Order(ent.Asc(upstreamaccountpool.FieldPriority), ent.Asc(upstreamaccountpool.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}
	if len(pools) == 0 {
		return nil, nil, nil
	}

	matchedPools := make([]*ent.UpstreamAccountPool, 0, len(pools))
	for _, pool := range pools {
		if upstreamAccountPoolMatches(pool, input.ModelID, input.ProjectID) {
			matchedPools = append(matchedPools, pool)
		}
	}
	if len(matchedPools) == 0 {
		return nil, nil, nil
	}

	poolIDs := make([]int, 0, len(matchedPools))
	for _, pool := range matchedPools {
		poolIDs = append(poolIDs, pool.ID)
	}

	accounts, err := s.entFromContext(ctx).UpstreamAccount.Query().
		Where(
			upstreamaccount.ChannelIDEQ(input.ChannelID),
			upstreamaccount.PoolIDIn(poolIDs...),
			upstreamaccount.StatusNEQ(upstreamaccount.StatusArchived),
		).
		Order(ent.Asc(upstreamaccount.FieldPriority), ent.Desc(upstreamaccount.FieldWeight), ent.Asc(upstreamaccount.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	candidates := make([]*ent.UpstreamAccount, 0, len(accounts))
	reasons := make([]string, 0, len(accounts))
	for _, account := range accounts {
		if _, excluded := input.ExcludeAccountIDs[account.ID]; excluded {
			reasons = append(reasons, fmt.Sprintf("%s skipped after previous attempt", account.Name))
			continue
		}
		if account.CredentialType != upstreamaccount.CredentialTypeAPIKey {
			reasons = append(reasons, fmt.Sprintf("%s uses unsupported credential type %s", account.Name, account.CredentialType))
			continue
		}
		if strings.TrimSpace(account.Credentials.APIKey) == "" {
			reasons = append(reasons, fmt.Sprintf("%s has no api key credential", account.Name))
			continue
		}
		if ok, reason := s.AccountEligibility(account, now); !ok {
			reasons = append(reasons, fmt.Sprintf("%s: %s", account.Name, reason))
			continue
		}
		if !s.accountHasCapacity(account) {
			reasons = append(reasons, fmt.Sprintf("%s concurrency limit reached", account.Name))
			continue
		}
		candidates = append(candidates, account)
	}

	if len(candidates) == 0 {
		reason := "no eligible upstream account"
		if len(reasons) > 0 {
			reason = strings.Join(reasons, "; ")
		}

		return nil, nil, &UpstreamAccountPoolExhaustedError{
			ChannelID: input.ChannelID,
			ModelID:   input.ModelID,
			Reason:    reason,
		}
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return s.compareAccountCandidates(candidates[i], candidates[j])
	})

	selected := candidates[0]
	release := s.acquireAccount(selected)
	if err := s.entFromContext(ctx).UpstreamAccount.UpdateOneID(selected.ID).SetLastUsedAt(now).Exec(ctx); err != nil {
		release()
		return nil, nil, err
	}

	return selected, release, nil
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

func (s *UpstreamAccountService) MarkAccountSuccess(ctx context.Context, accountID int, latencyMs int64) error {
	if accountID <= 0 {
		return nil
	}

	s.mu.Lock()
	rt := s.runtimeForLocked(accountID)
	rt.successes++
	if latencyMs > 0 {
		if rt.latencyEWMA <= 0 {
			rt.latencyEWMA = float64(latencyMs)
		} else {
			rt.latencyEWMA = rt.latencyEWMA*0.8 + float64(latencyMs)*0.2
		}
	}
	s.mu.Unlock()

	return s.entFromContext(ctx).UpstreamAccount.UpdateOneID(accountID).
		ClearErrorMessage().
		ClearCooldownReason().
		ClearRateLimitResetAt().
		ClearOverloadUntil().
		ClearCooldownUntil().
		Exec(ctx)
}

func (s *UpstreamAccountService) MarkAccountFailure(ctx context.Context, input UpstreamAccountFailureInput) error {
	if input.AccountID <= 0 {
		return nil
	}

	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		message = fmt.Sprintf("upstream account request failed with status %d", input.StatusCode)
	}

	s.mu.Lock()
	rt := s.runtimeForLocked(input.AccountID)
	rt.failures++
	s.mu.Unlock()

	update := s.entFromContext(ctx).UpstreamAccount.UpdateOneID(input.AccountID).
		SetErrorMessage(message)

	switch {
	case input.StatusCode == 401 || input.StatusCode == 403:
		update.SetStatus(upstreamaccount.StatusError).
			SetSchedulable(false).
			SetCooldownUntil(now.Add(24 * time.Hour)).
			SetCooldownReason("authentication failed")
	case input.StatusCode == 429:
		update.SetRateLimitResetAt(now.Add(defaultAccountRateLimitCooldown)).
			SetCooldownReason("rate limited")
	case input.StatusCode == 529 || (input.StatusCode >= 500 && input.StatusCode <= 599):
		update.SetOverloadUntil(now.Add(defaultAccountOverloadCooldown)).
			SetCooldownReason("provider overloaded")
	default:
		update.SetCooldownUntil(now.Add(defaultAccountNetworkCooldown)).
			SetCooldownReason("network or transport error")
	}

	return update.Exec(ctx)
}

func (s *UpstreamAccountService) accountHasCapacity(account *ent.UpstreamAccount) bool {
	if account == nil || account.ConcurrencyLimit <= 0 {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.runtimeForLocked(account.ID).inFlight < account.ConcurrencyLimit
}

func (s *UpstreamAccountService) acquireAccount(account *ent.UpstreamAccount) func() {
	if account == nil {
		return func() {}
	}

	s.mu.Lock()
	rt := s.runtimeForLocked(account.ID)
	rt.inFlight++
	s.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			rt := s.runtimeForLocked(account.ID)
			if rt.inFlight > 0 {
				rt.inFlight--
			}
		})
	}
}

func (s *UpstreamAccountService) compareAccountCandidates(left, right *ent.UpstreamAccount) bool {
	if left.Priority != right.Priority {
		return left.Priority < right.Priority
	}

	leftLoad, leftErrorRate, leftLatency := s.accountRuntimeSnapshot(left.ID)
	rightLoad, rightErrorRate, rightLatency := s.accountRuntimeSnapshot(right.ID)

	if leftLoad != rightLoad {
		return leftLoad < rightLoad
	}
	if leftErrorRate != rightErrorRate {
		return leftErrorRate < rightErrorRate
	}
	if leftLatency != rightLatency {
		return leftLatency < rightLatency
	}
	if left.Weight != right.Weight {
		return left.Weight > right.Weight
	}
	if left.LastUsedAt == nil && right.LastUsedAt != nil {
		return true
	}
	if left.LastUsedAt != nil && right.LastUsedAt == nil {
		return false
	}
	if left.LastUsedAt != nil && right.LastUsedAt != nil && !left.LastUsedAt.Equal(*right.LastUsedAt) {
		return left.LastUsedAt.Before(*right.LastUsedAt)
	}

	return left.ID < right.ID
}

func (s *UpstreamAccountService) accountRuntimeSnapshot(accountID int) (load int, errorRate float64, latency float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rt := s.runtimeForLocked(accountID)
	total := rt.successes + rt.failures
	if total > 0 {
		errorRate = float64(rt.failures) / float64(total)
	}

	return rt.inFlight, errorRate, rt.latencyEWMA
}

func (s *UpstreamAccountService) runtimeForLocked(accountID int) *upstreamAccountRuntime {
	rt := s.runtime[accountID]
	if rt == nil {
		rt = &upstreamAccountRuntime{}
		s.runtime[accountID] = rt
	}
	return rt
}

func (s *UpstreamAccountService) ChannelUsesCredentialFallback(ctx context.Context, channelID int) (bool, error) {
	count, err := s.entFromContext(ctx).UpstreamAccountPool.Query().
		Where(
			upstreamaccountpool.ChannelIDEQ(channelID),
			upstreamaccountpool.StatusEQ(upstreamaccountpool.StatusEnabled),
		).
		Count(ctx)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

func upstreamAccountPoolMatches(pool *ent.UpstreamAccountPool, modelID string, projectID int) bool {
	if pool == nil {
		return false
	}

	if len(pool.ProjectIds) > 0 {
		if projectID <= 0 {
			return false
		}
		projectMatched := false
		for _, id := range pool.ProjectIds {
			if id == projectID {
				projectMatched = true
				break
			}
		}
		if !projectMatched {
			return false
		}
	}

	if len(pool.ModelPatterns) == 0 {
		return true
	}

	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		return false
	}

	for _, pattern := range pool.ModelPatterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if pattern == "*" || strings.EqualFold(pattern, modelID) {
			return true
		}
		if matched, err := path.Match(pattern, modelID); err == nil && matched {
			return true
		}
	}

	return false
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
