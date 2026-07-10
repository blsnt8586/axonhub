package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billinghold"
	"github.com/looplj/axonhub/internal/ent/billingoutbox"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/pipeline"
	"github.com/looplj/axonhub/llm/pipeline/stream"
	"github.com/looplj/axonhub/llm/transformer/openai"
)

const commercialGatewayModel = "gpt-commercial"

type commercialGatewayFixture struct {
	client              *ent.Client
	ctx                 context.Context
	project             *ent.Project
	user                *ent.User
	apiKey              *ent.APIKey
	account             *ent.BillingAccount
	channel             *ent.Channel
	executor            *mockExecutor
	orchestrator        *ChatCompletionOrchestrator
	accountService      *biz.BillingAccountService
	ledgerService       *biz.LedgerService
	pricingService      *biz.PricingService
	subscriptionService *biz.SubscriptionService
	outboxWorker        *biz.BillingOutboxWorker
}

func TestGatewayCommercialLifecycleEnforceChargesUserWalletWithProjectPriceRule(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeEnforce)
	fixture.createPriceRule(t, billingpricerule.ScopeTypeGlobal, 0, "1", "global-v1")
	fixture.createPriceRule(t, billingpricerule.ScopeTypeProject, fixture.project.ID, "2", "project-v2")
	fixture.creditWallet(t, "10", "gateway-project-price-credit")

	result, err := fixture.process(t, 1_000_000, 500_000)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, int64(1), fixture.executor.requestCalls.Load())

	record, err := fixture.client.UsageBillingRecord.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Equal(t, fixture.user.ID, record.UserID)
	require.Equal(t, fixture.project.ID, record.ProjectID)
	require.Equal(t, "project-v2", record.PriceReferenceID)
	require.Equal(t, int64(3_000_000), record.ChargeAmountMicros)
	require.NotZero(t, record.LedgerTransactionID)

	account, err := fixture.client.BillingAccount.Get(fixture.ctx, fixture.account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(7_000_000), account.BalanceMicros)
	require.Zero(t, account.HeldBalanceMicros)

	hold, err := fixture.client.BillingHold.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, billinghold.StatusCaptured, hold.Status)
	require.Equal(t, int64(3_000_000), hold.CapturedAmountMicros)
	require.Equal(t, fixture.user.ID, hold.UserID)

	outbox, err := fixture.client.BillingOutbox.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusDone, outbox.Status)
	require.Equal(t, 1, outbox.Attempts)
}

func TestGatewayCommercialLifecycleEnforceBlocksBeforeUpstream(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeEnforce)

	_, err := fixture.process(t, 10, 5)
	require.Error(t, err)
	var responseErr *llm.ResponseError
	require.True(t, errors.As(err, &responseErr))
	require.Equal(t, http.StatusPaymentRequired, responseErr.StatusCode)
	require.Equal(t, string(biz.AdmissionCodeInsufficientBalance), responseErr.Detail.Code)
	require.Zero(t, fixture.executor.requestCalls.Load())

	requestCount, countErr := fixture.client.Request.Query().Count(fixture.ctx)
	require.NoError(t, countErr)
	require.Zero(t, requestCount)
	usageCount, countErr := fixture.client.UsageLog.Query().Count(fixture.ctx)
	require.NoError(t, countErr)
	require.Zero(t, usageCount)
}

func TestGatewayCommercialLifecycleWarnRoutesAndRetriesFailedBilling(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeWarn)
	fixture.createPriceRule(t, billingpricerule.ScopeTypeProject, fixture.project.ID, "2", "warn-project-v1")

	result, err := fixture.process(t, 1_000_000, 500_000)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Equal(t, int64(1), fixture.executor.requestCalls.Load())

	outbox, err := fixture.client.BillingOutbox.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusFailed, outbox.Status)
	require.Equal(t, 1, outbox.Attempts)
	require.NotEmpty(t, outbox.LastError)

	recordCount, err := fixture.client.UsageBillingRecord.Query().Count(fixture.ctx)
	require.NoError(t, err)
	require.Zero(t, recordCount)

	fixture.creditWallet(t, "5", "warn-retry-credit")
	_, err = fixture.client.BillingOutbox.UpdateOneID(outbox.ID).
		SetNextAttemptAt(time.Now().UTC().Add(-time.Second)).
		Save(fixture.ctx)
	require.NoError(t, err)

	processed, err := fixture.outboxWorker.ProcessDueOutbox(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, 1, processed)

	record, err := fixture.client.UsageBillingRecord.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Equal(t, int64(3_000_000), record.ChargeAmountMicros)
	outbox, err = fixture.client.BillingOutbox.Get(fixture.ctx, outbox.ID)
	require.NoError(t, err)
	require.Equal(t, billingoutbox.StatusDone, outbox.Status)
	require.Equal(t, 2, outbox.Attempts)

	account, err := fixture.client.BillingAccount.Get(fixture.ctx, fixture.account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(2_000_000), account.BalanceMicros)
}

func TestGatewayCommercialLifecycleSubscriptionPrecedesWallet(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeEnforce)
	fixture.createPriceRule(t, billingpricerule.ScopeTypeProject, fixture.project.ID, "1", "subscription-project-v1")

	plan, err := fixture.subscriptionService.SavePlan(fixture.ctx, biz.SaveSubscriptionPlanInput{
		Name:              "Gateway Included",
		PeriodDays:        30,
		IncludedAmount:    decimal.RequireFromString("2"),
		SupportedModelIDs: []string{commercialGatewayModel},
		Currency:          "CNY",
	})
	require.NoError(t, err)
	subscription, err := fixture.subscriptionService.AdminAssign(fixture.ctx, biz.AdminAssignSubscriptionInput{
		UserID: fixture.user.ID,
		PlanID: plan.ID,
	})
	require.NoError(t, err)

	result, err := fixture.process(t, 1_000_000, 500_000)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)

	record, err := fixture.client.UsageBillingRecord.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusSkipped, record.Status)
	require.Equal(t, subscription.ID, record.UserSubscriptionID)
	require.Equal(t, int64(1_500_000), record.ChargeAmountMicros)
	require.Zero(t, record.LedgerTransactionID)

	account, err := fixture.client.BillingAccount.Get(fixture.ctx, fixture.account.ID)
	require.NoError(t, err)
	require.Zero(t, account.BalanceMicros)
	require.Zero(t, account.HeldBalanceMicros)
	holdCount, err := fixture.client.BillingHold.Query().Count(fixture.ctx)
	require.NoError(t, err)
	require.Zero(t, holdCount)

	subscription, err = fixture.client.UserSubscription.Get(fixture.ctx, subscription.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1_500_000), subscription.UsedAmountMicros)
}

func TestGatewayCommercialLifecycleExhaustedSubscriptionFallsBackToWallet(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeEnforce)
	fixture.createPriceRule(t, billingpricerule.ScopeTypeProject, fixture.project.ID, "1", "wallet-fallback-v1")
	fixture.creditWallet(t, "5", "wallet-fallback-credit")

	plan, err := fixture.subscriptionService.SavePlan(fixture.ctx, biz.SaveSubscriptionPlanInput{
		Name:              "Gateway Exhausted",
		PeriodDays:        30,
		IncludedAmount:    decimal.RequireFromString("1"),
		SupportedModelIDs: []string{commercialGatewayModel},
		Currency:          "CNY",
	})
	require.NoError(t, err)
	subscription, err := fixture.subscriptionService.AdminAssign(fixture.ctx, biz.AdminAssignSubscriptionInput{
		UserID: fixture.user.ID,
		PlanID: plan.ID,
	})
	require.NoError(t, err)
	_, err = fixture.client.UserSubscription.UpdateOneID(subscription.ID).
		SetUsedAmountMicros(subscription.IncludedAmountMicros).
		Save(fixture.ctx)
	require.NoError(t, err)

	result, err := fixture.process(t, 1_000_000, 500_000)
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)

	record, err := fixture.client.UsageBillingRecord.Query().Only(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, record.Status)
	require.Zero(t, record.UserSubscriptionID)
	require.Equal(t, int64(1_500_000), record.ChargeAmountMicros)

	account, err := fixture.client.BillingAccount.Get(fixture.ctx, fixture.account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3_500_000), account.BalanceMicros)
	require.Zero(t, account.HeldBalanceMicros)
	subscription, err = fixture.client.UserSubscription.Get(fixture.ctx, subscription.ID)
	require.NoError(t, err)
	require.Equal(t, int64(1_000_000), subscription.UsedAmountMicros)
}

func TestGatewayCommercialLifecycleBillingPreservesUpstreamRetry(t *testing.T) {
	fixture := newCommercialGatewayFixture(t, biz.AdmissionModeEnforce)
	fixture.createPriceRule(t, billingpricerule.ScopeTypeProject, fixture.project.ID, "1", "retry-project-v1")
	fixture.creditWallet(t, "5", "retry-credit")
	require.NoError(t, fixture.orchestrator.SystemService.SetRetryPolicy(fixture.ctx, &biz.RetryPolicy{
		Enabled:                 true,
		MaxChannelRetries:       1,
		MaxSingleChannelRetries: 0,
		RetryDelayMs:            0,
		LoadBalancerStrategy:    biz.LoadBalancerStrategyAdaptive,
	}))

	secondChannel, err := fixture.client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Commercial Retry Channel").
		SetBaseURL("https://retry.example.com/v1").
		SetCredentials(objects.ChannelCredentials{APIKey: "retry-secret"}).
		SetSupportedModels([]string{commercialGatewayModel}).
		SetDefaultTestModel(commercialGatewayModel).
		Save(fixture.ctx)
	require.NoError(t, err)
	firstOutbound, err := openai.NewOutboundTransformer(fixture.channel.BaseURL, fixture.channel.Credentials.APIKey)
	require.NoError(t, err)
	secondOutbound, err := openai.NewOutboundTransformer(secondChannel.BaseURL, secondChannel.Credentials.APIKey)
	require.NoError(t, err)
	fixture.orchestrator.channelSelector = &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{
		{Channel: fixture.channel, Outbound: firstOutbound},
		{Channel: secondChannel, Outbound: secondOutbound},
	}, commercialGatewayModel)}

	executor := &sequenceExecutor{steps: []executorStep{
		{err: &httpclient.Error{StatusCode: http.StatusInternalServerError, Body: []byte(`{"error":{"message":"first account failed"}}`)}},
		{resp: commercialGatewayResponse(1_000_000, 500_000)},
	}}
	fixture.orchestrator.PipelineFactory = pipeline.NewFactory(executor)

	result, err := fixture.orchestrator.Process(fixture.ctx, buildTestRequest(commercialGatewayModel, "retry", false))
	require.NoError(t, err)
	require.NotNil(t, result.ChatCompletion)
	require.Len(t, executor.requests, 2)
	require.NotEqual(t, executor.requests[0].URL, executor.requests[1].URL)

	recordCount, err := fixture.client.UsageBillingRecord.Query().Count(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, 1, recordCount)
	transactionCount, err := fixture.client.LedgerTransaction.Query().
		Where().
		Count(fixture.ctx)
	require.NoError(t, err)
	require.Equal(t, 2, transactionCount, "one recharge and one usage debit are expected")
	account, err := fixture.client.BillingAccount.Get(fixture.ctx, fixture.account.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3_500_000), account.BalanceMicros)
}

func newCommercialGatewayFixture(t *testing.T, mode biz.AdmissionMode) *commercialGatewayFixture {
	t.Helper()

	databaseName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	client := enttest.NewEntClient(t, "sqlite3", fmt.Sprintf("file:%s?mode=memory&_fk=0", databaseName))
	t.Cleanup(func() { _ = client.Close() })
	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)

	project := createTestProject(t, ctx, client)
	user, err := client.User.Create().
		SetEmail(fmt.Sprintf("%s@example.com", strings.ToLower(databaseName))).
		SetPassword("test-password").
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetName("Commercial Gateway Key").
		SetKey("ah-" + strings.ToLower(databaseName)).
		SetProjectID(project.ID).
		SetUserID(user.ID).
		Save(ctx)
	require.NoError(t, err)
	channelRow := createCommercialGatewayChannel(t, ctx, client, "Commercial Primary Channel", "https://primary.example.com/v1")

	channelService, requestService, systemService, usageLogService := setupTestServices(t, client)
	accountService := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	account, err := accountService.GetOrCreateForSubject(ctx, biz.UserBillingSubject(user.ID))
	require.NoError(t, err)
	ledgerService := biz.NewLedgerService(biz.LedgerServiceParams{Ent: client})
	pricingService := biz.NewPricingService(biz.PricingServiceParams{Ent: client})
	subscriptionService := biz.NewSubscriptionService(biz.SubscriptionServiceParams{
		Ent:                   client,
		BillingAccountService: accountService,
		LedgerService:         ledgerService,
	})
	billingConfig := biz.BillingConfig{
		Mode:                    mode,
		Subject:                 biz.BillingSubjectTypeUser,
		Currency:                "CNY",
		HoldDefaultAmount:       decimal.RequireFromString("0.000001"),
		OutboxRetryDelaySeconds: 1,
	}
	billingHoldService := biz.NewBillingHoldService(biz.BillingHoldServiceParams{
		Ent:                   client,
		LedgerService:         ledgerService,
		Config:                billingConfig,
		BillingAccountService: accountService,
		PricingService:        pricingService,
	})
	usageBillingProcessor := biz.NewUsageBillingProcessor(biz.UsageBillingProcessorParams{
		Config:                billingConfig,
		Ent:                   client,
		PricingService:        pricingService,
		BillingAccountService: accountService,
		LedgerService:         ledgerService,
		BillingHoldService:    billingHoldService,
		SubscriptionService:   subscriptionService,
	})
	admissionService := biz.NewAdmissionService(biz.AdmissionServiceParams{
		Config:                billingConfig,
		BillingAccountService: accountService,
		SubscriptionService:   subscriptionService,
	})
	outboxWorker := biz.NewBillingOutboxWorker(biz.BillingOutboxWorkerParams{
		Config:                billingConfig,
		Ent:                   client,
		UsageBillingProcessor: usageBillingProcessor,
	})

	executor := &mockExecutor{response: commercialGatewayResponse(1_000_000, 500_000)}
	outbound, err := openai.NewOutboundTransformer(channelRow.BaseURL, channelRow.Credentials.APIKey)
	require.NoError(t, err)
	selector := &staticChannelSelector{candidates: channelsToTestCandidates([]*biz.Channel{{
		Channel:  channelRow,
		Outbound: outbound,
	}}, commercialGatewayModel)}
	orchestrator := &ChatCompletionOrchestrator{
		channelSelector:       selector,
		Inbound:               openai.NewInboundTransformer(),
		RequestService:        requestService,
		ChannelService:        channelService,
		PromptProvider:        &stubPromptProvider{},
		SystemService:         systemService,
		UsageLogService:       usageLogService,
		UsageBillingProcessor: usageBillingProcessor,
		BillingHoldService:    billingHoldService,
		AdmissionService:      admissionService,
		PipelineFactory:       pipeline.NewFactory(executor),
		ModelMapper:           NewModelMapper(),
		modelCircuitBreaker:   biz.NewModelCircuitBreaker(),
		channelLimiterManager: NewChannelLimiterManager(),
		Middlewares: []pipeline.Middleware{
			stream.EnsureUsage(),
		},
	}

	ctx = contexts.WithProjectID(ctx, project.ID)
	ctx = contexts.WithAPIKey(ctx, apiKey)

	return &commercialGatewayFixture{
		client:              client,
		ctx:                 ctx,
		project:             project,
		user:                user,
		apiKey:              apiKey,
		account:             account,
		channel:             channelRow,
		executor:            executor,
		orchestrator:        orchestrator,
		accountService:      accountService,
		ledgerService:       ledgerService,
		pricingService:      pricingService,
		subscriptionService: subscriptionService,
		outboxWorker:        outboxWorker,
	}
}

func (f *commercialGatewayFixture) process(t *testing.T, promptTokens, completionTokens int) (ChatCompletionResult, error) {
	t.Helper()
	f.executor.response = commercialGatewayResponse(promptTokens, completionTokens)
	return f.orchestrator.Process(f.ctx, buildTestRequest(commercialGatewayModel, "commercial acceptance", false))
}

func (f *commercialGatewayFixture) createPriceRule(t *testing.T, scopeType billingpricerule.ScopeType, scopeID int, unitPrice, referenceID string) {
	t.Helper()
	_, err := f.pricingService.SaveBillingPriceRule(f.ctx, biz.SaveBillingPriceRuleInput{
		ScopeType:    scopeType,
		ScopeID:      scopeID,
		ModelPattern: commercialGatewayModel,
		Price:        commercialGatewayPrice(unitPrice),
		Currency:     "CNY",
		ReferenceID:  referenceID,
	})
	require.NoError(t, err)
}

func (f *commercialGatewayFixture) creditWallet(t *testing.T, amount, idempotencyKey string) {
	t.Helper()
	_, err := f.ledgerService.Credit(
		f.ctx,
		f.account.ID,
		decimal.RequireFromString(amount),
		ledgertransaction.TypePaymentRecharge,
		idempotencyKey,
	)
	require.NoError(t, err)
}

func createCommercialGatewayChannel(t *testing.T, ctx context.Context, client *ent.Client, name, baseURL string) *ent.Channel {
	t.Helper()
	channelRow, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetBaseURL(baseURL).
		SetCredentials(objects.ChannelCredentials{APIKey: "commercial-secret"}).
		SetSupportedModels([]string{commercialGatewayModel}).
		SetDefaultTestModel(commercialGatewayModel).
		Save(ctx)
	require.NoError(t, err)
	return channelRow
}

func commercialGatewayResponse(promptTokens, completionTokens int) *httpclient.Response {
	return &httpclient.Response{
		StatusCode: http.StatusOK,
		Body: buildMockOpenAIResponse(
			"chatcmpl-commercial",
			commercialGatewayModel,
			"ok",
			promptTokens,
			completionTokens,
		),
		Headers: http.Header{"Content-Type": []string{"application/json"}},
	}
}

func commercialGatewayPrice(unitPrice string) objects.ModelPrice {
	price := decimal.RequireFromString(unitPrice)
	return objects.ModelPrice{Items: []objects.ModelPriceItem{
		{
			ItemCode: objects.PriceItemCodeUsage,
			Pricing: objects.Pricing{
				Mode:         objects.PricingModeUsagePerUnit,
				UsagePerUnit: &price,
			},
		},
		{
			ItemCode: objects.PriceItemCodeCompletion,
			Pricing: objects.Pricing{
				Mode:         objects.PricingModeUsagePerUnit,
				UsagePerUnit: &price,
			},
		},
	}}
}
