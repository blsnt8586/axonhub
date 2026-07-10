package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/model"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func TestUserPlaygroundStateIsConsumerSafeAndUsesEffectivePriceRule(t *testing.T) {
	t.Parallel()

	fixture := newUserPlaygroundFixture(t, "user_playground_state")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "playground@example.com")
	other := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "other-playground@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Playground")
	addUserAPIKeyMembership(t, fixture.ctx, fixture.client, other.ID, workspace.ID)
	owned := createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Owned", "ah-owned-playground", apikey.TypePersonal)
	createUserAPIKeyRow(t, fixture.ctx, fixture.client, other.ID, workspace.ID, "Foreign", "ah-foreign-playground", apikey.TypePersonal)

	_, err := fixture.client.Channel.Create().
		SetName("Private upstream").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-consumer"}).
		SetDefaultTestModel("gpt-consumer").
		Save(fixture.ctx)
	require.NoError(t, err)
	_, err = fixture.client.Model.Create().
		SetDeveloper("OpenAI").
		SetModelID("gpt-consumer").
		SetType(model.TypeChat).
		SetName("Consumer Chat").
		SetIcon("OpenAI").
		SetGroup("openai").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetStatus(model.StatusEnabled).
		Save(fixture.ctx)
	require.NoError(t, err)
	_, err = fixture.client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("gpt-consumer").
		SetPrice(testModelPrice("1")).
		SetCurrency("CNY").
		SetReferenceID("global-consumer").
		Save(fixture.ctx)
	require.NoError(t, err)
	_, err = fixture.client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeProject).
		SetScopeID(workspace.ID).
		SetModelPattern("gpt-consumer").
		SetPrice(testModelPrice("2")).
		SetCurrency("CNY").
		SetReferenceID("project-consumer").
		Save(fixture.ctx)
	require.NoError(t, err)

	state, err := fixture.service.State(contexts.WithUser(fixture.ctx, viewer), viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.True(t, state.CanSend)
	require.Equal(t, UserPlaygroundBlockReasonNone, state.BlockReason)
	require.Len(t, state.APIKeys, 1)
	require.Equal(t, workspaceObjectGUID(owned.ID, ent.TypeAPIKey), state.APIKeys[0].ID)
	require.True(t, state.APIKeys[0].Usable)
	require.Len(t, state.Models, 1)
	require.Equal(t, "gpt-consumer", state.Models[0].ModelID)
	require.Equal(t, "chat", state.Models[0].Modality)
	require.Equal(t, UserPlaygroundModelAvailabilityAvailable, state.Models[0].Availability)
	require.Equal(t, "project", state.Models[0].PriceRule.Scope)
	require.Equal(t, "gpt-consumer", state.Models[0].PriceRule.Pattern)
	require.Equal(t, testModelPrice("2"), *state.Models[0].Price)
}

func TestUserPlaygroundStateReturnsDeterministicBlockingReasons(t *testing.T) {
	t.Parallel()

	fixture := newUserPlaygroundFixture(t, "user_playground_blocks")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "blocks@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Blocks")
	viewerCtx := contexts.WithUser(fixture.ctx, viewer)

	state, err := fixture.service.State(viewerCtx, viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, UserPlaygroundBlockReasonNoModel, state.BlockReason)

	_, err = fixture.client.Channel.Create().
		SetName("Available upstream").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-blocks"}).
		SetDefaultTestModel("gpt-blocks").
		Save(fixture.ctx)
	require.NoError(t, err)
	state, err = fixture.service.State(viewerCtx, viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, UserPlaygroundBlockReasonNoEnabledKey, state.BlockReason)

	disabled := createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Disabled", "ah-disabled-playground", apikey.TypePersonal)
	_, err = fixture.client.APIKey.UpdateOneID(disabled.ID).SetStatus(apikey.StatusDisabled).Save(fixture.ctx)
	require.NoError(t, err)
	state, err = fixture.service.State(viewerCtx, viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, UserPlaygroundBlockReasonKeyDisabled, state.BlockReason)
	require.Equal(t, UserPlaygroundKeyBlockReasonDisabled, state.APIKeys[0].BlockReason)

	_, err = fixture.client.APIKey.UpdateOneID(disabled.ID).
		SetStatus(apikey.StatusEnabled).
		SetExpiresAt(time.Now().UTC().Add(-time.Minute)).
		Save(fixture.ctx)
	require.NoError(t, err)
	state, err = fixture.service.State(viewerCtx, viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, UserPlaygroundBlockReasonKeyExpired, state.BlockReason)
	require.Equal(t, UserPlaygroundKeyBlockReasonExpired, state.APIKeys[0].BlockReason)
}

func TestUserPlaygroundPrepareChatRejectsForeignAndUnavailableResources(t *testing.T) {
	t.Parallel()

	fixture := newUserPlaygroundFixture(t, "user_playground_prepare")
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "prepare@example.com")
	other := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "prepare-other@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Prepare")
	addUserAPIKeyMembership(t, fixture.ctx, fixture.client, other.ID, workspace.ID)
	owned := createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Owned", "ah-prepare-owned", apikey.TypePersonal)
	foreign := createUserAPIKeyRow(t, fixture.ctx, fixture.client, other.ID, workspace.ID, "Foreign", "ah-prepare-foreign", apikey.TypePersonal)
	viewerCtx := contexts.WithUser(fixture.ctx, viewer)

	_, err := fixture.service.PrepareChat(viewerCtx, viewer, workspace.ID, foreign.ID, "gpt-missing", "203.0.113.10")
	require.ErrorIs(t, err, ErrUserPlaygroundDenied)

	_, err = fixture.service.PrepareChat(viewerCtx, viewer, workspace.ID, owned.ID, "gpt-missing", "203.0.113.10")
	require.ErrorIs(t, err, ErrUserPlaygroundModelUnavailable)

	_, err = fixture.client.Channel.Create().
		SetName("Prepare upstream").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-ready"}).
		SetDefaultTestModel("gpt-ready").
		Save(fixture.ctx)
	require.NoError(t, err)
	prepared, err := fixture.service.PrepareChat(viewerCtx, viewer, workspace.ID, owned.ID, "gpt-ready", "203.0.113.10")
	require.NoError(t, err)
	require.Equal(t, owned.ID, prepared.APIKey.ID)
	require.Equal(t, workspace.ID, prepared.ProjectID)
}

func TestUserPlaygroundStateReportsInsufficientBalanceInEnforceMode(t *testing.T) {
	t.Parallel()

	fixture := newUserPlaygroundFixtureWithConfig(t, "user_playground_balance", BillingConfig{Mode: AdmissionModeEnforce, Currency: "CNY"})
	viewer := createUserAPIKeyUser(t, fixture.ctx, fixture.client, "balance-playground@example.com")
	workspace := createUserAPIKeyProject(t, fixture.ctx, fixture.client, viewer.ID, "Balance")
	createUserAPIKeyRow(t, fixture.ctx, fixture.client, viewer.ID, workspace.ID, "Balance Key", "ah-balance-playground", apikey.TypePersonal)
	_, err := fixture.billingAccounts.GetOrCreateForSubject(fixture.ctx, UserBillingSubject(viewer.ID))
	require.NoError(t, err)
	_, err = fixture.client.Channel.Create().
		SetName("Balance upstream").
		SetType(channel.TypeOpenai).
		SetStatus(channel.StatusEnabled).
		SetCredentials(objects.ChannelCredentials{}).
		SetSupportedModels([]string{"gpt-balance"}).
		SetDefaultTestModel("gpt-balance").
		Save(fixture.ctx)
	require.NoError(t, err)

	state, err := fixture.service.State(contexts.WithUser(fixture.ctx, viewer), viewer, workspace.ID, "203.0.113.10", time.Now().UTC())
	require.NoError(t, err)
	require.False(t, state.CanSend)
	require.Equal(t, UserPlaygroundBlockReasonBalanceInsufficient, state.BlockReason)
}

type userPlaygroundFixture struct {
	client          *ent.Client
	ctx             context.Context
	service         *UserPlaygroundService
	billingAccounts *BillingAccountService
}

func newUserPlaygroundFixture(t *testing.T, name string) userPlaygroundFixture {
	return newUserPlaygroundFixtureWithConfig(t, name, BillingConfig{Mode: AdmissionModeDisabled, Currency: "CNY"})
}

func newUserPlaygroundFixtureWithConfig(t *testing.T, name string, config BillingConfig) userPlaygroundFixture {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(context.Background())
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	projectService := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := NewAPIKeyService(APIKeyServiceParams{Ent: client, CacheConfig: cacheConfig, ProjectService: projectService, KeyPrefix: "ah"})
	t.Cleanup(apiKeyService.Stop)
	billingAccountService := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	workspaceSummaryService := NewUserWorkspaceSummaryService(UserWorkspaceSummaryServiceParams{
		Ent: client, BillingAccountService: billingAccountService,
	})
	pricingService := NewPricingService(PricingServiceParams{Ent: client})
	userAPIKeyService := NewUserAPIKeyService(UserAPIKeyServiceParams{
		Ent: client, APIKeyService: apiKeyService, WorkspaceSummaryService: workspaceSummaryService, PricingService: pricingService,
	})
	admissionService := NewAdmissionService(AdmissionServiceParams{
		Config:                config,
		BillingAccountService: billingAccountService,
	})
	return userPlaygroundFixture{
		client:          client,
		ctx:             ctx,
		billingAccounts: billingAccountService,
		service: NewUserPlaygroundService(UserPlaygroundServiceParams{
			Ent: client, UserAPIKeyService: userAPIKeyService, WorkspaceSummaryService: workspaceSummaryService,
			PricingService: pricingService, AdmissionService: admissionService,
		}),
	}
}
