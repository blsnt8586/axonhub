package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/upstreamaccount"
	"github.com/looplj/axonhub/internal/objects"
)

func setupUpstreamAccountServiceTest(t *testing.T, name string) (context.Context, *ent.Client, *UpstreamAccountService, *ent.Channel) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	svc := NewUpstreamAccountService(UpstreamAccountServiceParams{Ent: client})

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Pool Test Channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "fallback-key"}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return ctx, client, svc, ch
}

func TestUpstreamAccountServiceFallbackWhenNoPoolConfigured(t *testing.T) {
	ctx, client, svc, ch := setupUpstreamAccountServiceTest(t, "upstream_account_fallback")
	defer client.Close()

	fallback, err := svc.ChannelUsesCredentialFallback(ctx, ch.ID)
	require.NoError(t, err)
	require.True(t, fallback)

	_, err = svc.CreatePool(ctx, CreateUpstreamAccountPoolParams{
		ChannelID: ch.ID,
		Name:      "default",
	})
	require.NoError(t, err)

	fallback, err = svc.ChannelUsesCredentialFallback(ctx, ch.ID)
	require.NoError(t, err)
	require.False(t, fallback)
}

func TestUpstreamAccountServiceEligibilityFiltersUnschedulableAccounts(t *testing.T) {
	ctx, client, svc, ch := setupUpstreamAccountServiceTest(t, "upstream_account_eligibility")
	defer client.Close()

	now := time.Now()
	creds := objects.UpstreamAccountCredentials{APIKey: "sk-valid"}

	valid, err := svc.CreateAccount(ctx, CreateUpstreamAccountParams{
		ChannelID:      ch.ID,
		Name:           "valid",
		Credentials:    creds,
		Status:         upstreamaccount.StatusActive,
		Schedulable:    true,
		RateMultiplier: 1,
	})
	require.NoError(t, err)

	cases := []CreateUpstreamAccountParams{
		{
			ChannelID:      ch.ID,
			Name:           "disabled",
			Credentials:    creds,
			Status:         upstreamaccount.StatusDisabled,
			Schedulable:    true,
			RateMultiplier: 1,
		},
		{
			ChannelID:      ch.ID,
			Name:           "manual-paused",
			Credentials:    creds,
			Status:         upstreamaccount.StatusActive,
			Schedulable:    false,
			RateMultiplier: 1,
		},
		{
			ChannelID:      ch.ID,
			Name:           "expired",
			Credentials:    creds,
			Status:         upstreamaccount.StatusActive,
			Schedulable:    true,
			RateMultiplier: 1,
			ExpiresAt:      ptrTime(now.Add(-time.Minute)),
		},
		{
			ChannelID:        ch.ID,
			Name:             "quota-exhausted",
			Credentials:      creds,
			Status:           upstreamaccount.StatusActive,
			Schedulable:      true,
			RateMultiplier:   1,
			QuotaLimitMicros: 100,
			QuotaUsedMicros:  100,
		},
		{
			ChannelID:        ch.ID,
			Name:             "rate-limited",
			Credentials:      creds,
			Status:           upstreamaccount.StatusActive,
			Schedulable:      true,
			RateMultiplier:   1,
			RateLimitResetAt: ptrTime(now.Add(time.Hour)),
		},
		{
			ChannelID:      ch.ID,
			Name:           "overloaded",
			Credentials:    creds,
			Status:         upstreamaccount.StatusActive,
			Schedulable:    true,
			RateMultiplier: 1,
			OverloadUntil:  ptrTime(now.Add(time.Hour)),
		},
		{
			ChannelID:      ch.ID,
			Name:           "cooling",
			Credentials:    creds,
			Status:         upstreamaccount.StatusActive,
			Schedulable:    true,
			RateMultiplier: 1,
			CooldownUntil:  ptrTime(now.Add(time.Hour)),
			CooldownReason: ptrString("network error"),
		},
	}

	for _, tc := range cases {
		_, err := svc.CreateAccount(ctx, tc)
		require.NoError(t, err)
	}

	eligible, err := svc.EligibleAccounts(ctx, ch.ID, now)
	require.NoError(t, err)
	require.Len(t, eligible, 1)
	require.Equal(t, valid.ID, eligible[0].ID)

	_, reason := svc.AccountEligibility(eligible[0], now)
	require.Empty(t, reason)
}

func TestUpstreamAccountServicePreservesCredentialsOnEmptyUpdate(t *testing.T) {
	ctx, client, svc, ch := setupUpstreamAccountServiceTest(t, "upstream_account_credentials_update")
	defer client.Close()

	account, err := svc.CreateAccount(ctx, CreateUpstreamAccountParams{
		ChannelID:      ch.ID,
		Name:           "write-only",
		Credentials:    objects.UpstreamAccountCredentials{APIKey: "sk-original"},
		Status:         upstreamaccount.StatusActive,
		Schedulable:    true,
		RateMultiplier: 1,
	})
	require.NoError(t, err)

	emptyCreds := objects.UpstreamAccountCredentials{}
	updated, err := svc.UpdateAccount(ctx, account.ID, UpdateUpstreamAccountParams{
		Name:        ptrString("renamed"),
		Credentials: &emptyCreds,
	})
	require.NoError(t, err)
	require.Equal(t, "renamed", updated.Name)
	require.Equal(t, "sk-original", updated.Credentials.APIKey)
}

func ptrTime(value time.Time) *time.Time {
	return &value
}

func ptrString(value string) *string {
	return &value
}
