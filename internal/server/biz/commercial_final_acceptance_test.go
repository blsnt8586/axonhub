package biz

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billingpricerule"
	"github.com/looplj/axonhub/internal/ent/commercialsetting"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/ent/requestexecution"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
)

func TestCommercialOperationsFinalAcceptanceUserCommercialLifecycle(t *testing.T) {
	client, ctx, registrationSvc := setupRegistrationServiceTest(t)

	billingAccountSvc := registrationSvc.BillingAccountService
	ledgerSvc := registrationSvc.LedgerService
	pricingSvc := NewPricingService(PricingServiceParams{Ent: client})
	aggregateSvc := NewUsageAggregateService(UsageAggregateServiceParams{Ent: client})
	commercialLimitSvc := NewAPIKeyCommercialLimitService(APIKeyCommercialLimitServiceParams{
		Ent:           client,
		SystemService: registrationSvc.SystemService,
	})
	subscriptionSvc := NewSubscriptionService(SubscriptionServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
	})
	billingHoldSvc := NewBillingHoldService(BillingHoldServiceParams{
		Ent:                   client,
		LedgerService:         ledgerSvc,
		Config:                BillingConfig{Mode: AdmissionModeEnforce},
		BillingAccountService: billingAccountSvc,
		PricingService:        pricingSvc,
	})
	paymentSvc := NewPaymentService(PaymentServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
		ProviderRegistry:      NewPaymentProviderRegistry(),
	})
	redeemSvc := NewRedeemCodeService(RedeemCodeServiceParams{
		Ent:                   client,
		BillingAccountService: billingAccountSvc,
		LedgerService:         ledgerSvc,
	})
	processor := NewUsageBillingProcessor(UsageBillingProcessorParams{
		Config:                 BillingConfig{Mode: AdmissionModeEnforce},
		Ent:                    client,
		PricingService:         pricingSvc,
		BillingAccountService:  billingAccountSvc,
		LedgerService:          ledgerSvc,
		BillingHoldService:     billingHoldSvc,
		CommercialLimitService: commercialLimitSvc,
		SubscriptionService:    subscriptionSvc,
		UsageAggregateService:  aggregateSvc,
	})
	admissionSvc := NewAdmissionService(AdmissionServiceParams{
		Config:                 BillingConfig{Mode: AdmissionModeEnforce, Currency: "CNY"},
		BillingAccountService:  billingAccountSvc,
		CommercialLimitService: commercialLimitSvc,
		SubscriptionService:    subscriptionSvc,
	})
	outboxWorker := NewBillingOutboxWorker(BillingOutboxWorkerParams{
		Config:                BillingConfig{Mode: AdmissionModeEnforce},
		Ent:                   client,
		UsageBillingProcessor: processor,
	})
	operationsSvc := NewCommercialOperationsService(CommercialOperationsServiceParams{
		Ent:                   client,
		PaymentService:        paymentSvc,
		BillingHoldService:    billingHoldSvc,
		SubscriptionService:   subscriptionSvc,
		BillingOutboxWorker:   outboxWorker,
		UsageAggregateService: aggregateSvc,
		BillingAuditService:   NewBillingAuditService(BillingAuditServiceParams{Ent: client}),
	})
	authSvc := NewAuthService(AuthServiceParams{
		SystemService: registrationSvc.SystemService,
		APIKeyService: registrationSvc.APIKeyService,
		Ent:           client,
	})

	require.NoError(t, registrationSvc.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:              true,
		SignupGrantAmount:    "5",
		CreateDefaultProject: true,
		CreateDefaultAPIKey:  true,
		DefaultProjectName:   "Starter",
		DefaultAPIKeyName:    "Starter Key",
	}))
	registered, err := registrationSvc.Register(ctx, RegisterUserInput{
		Email:    "commercial-final@example.com",
		Password: "Password1",
	})
	require.NoError(t, err)
	require.NotNil(t, registered.Project)
	require.NotNil(t, registered.APIKey)
	require.Equal(t, billingaccount.OwnerTypeUser, registered.BillingAccount.OwnerType)
	require.Equal(t, registered.User.ID, registered.BillingAccount.OwnerID)
	require.Equal(t, int64(5_000_000), registered.BillingAccount.BalanceMicros)

	authenticated, err := authSvc.AuthenticateUser(ctx, "commercial-final@example.com", "Password1")
	require.NoError(t, err)
	require.Equal(t, registered.User.ID, authenticated.ID)

	provider, err := paymentSvc.GetOrCreateSimulatedEPayProvider(ctx, "https://axon.example.com")
	require.NoError(t, err)
	require.NotContains(t, string(provider.Config), "axonhub-simulated-epay-secret")
	checkout, err := paymentSvc.CreateRechargeCheckout(ctx, CreateRechargeCheckoutInput{
		ProjectID:          registered.Project.ID,
		BillingSubject:     UserBillingSubject(registered.User.ID),
		ProviderInstanceID: &provider.ID,
		ProviderType:       paymentproviderinstance.ProviderTypeEpay,
		Amount:             decimal.RequireFromString("20"),
		Currency:           "CNY",
		Subject:            "Final acceptance recharge",
	})
	require.NoError(t, err)
	paidOrder, err := paymentSvc.HandleEPayNotify(ctx, HandleEPayNotifyInput{
		Params: NewSimulatedEPayNotifyFromCheckout(checkout.Params, "axonhub-simulated-epay-secret"),
	})
	require.NoError(t, err)
	require.Equal(t, registered.BillingAccount.ID, paidOrder.BillingAccountID)

	codes, err := redeemSvc.CreateRedeemCodes(ctx, CreateRedeemCodesInput{
		Count:    1,
		Amount:   decimal.RequireFromString("3"),
		Currency: "CNY",
		Notes:    "final acceptance grant",
		ActorID:  "owner",
	})
	require.NoError(t, err)
	_, err = redeemSvc.Redeem(ctx, RedeemCodeInput{Code: codes[0].Code, UserID: registered.User.ID})
	require.NoError(t, err)

	account, err := billingAccountSvc.GetBySubject(ctx, UserBillingSubject(registered.User.ID))
	require.NoError(t, err)
	require.Equal(t, int64(28_000_000), account.BalanceMicros)

	plan, err := subscriptionSvc.SavePlan(ctx, SaveSubscriptionPlanInput{
		Name:              "Final Acceptance",
		PeriodDays:        30,
		Price:             decimal.RequireFromString("4"),
		IncludedAmount:    decimal.RequireFromString("2"),
		SupportedModelIDs: []string{"gpt-aggregate"},
		Currency:          "CNY",
	})
	require.NoError(t, err)
	subscription, err := subscriptionSvc.PurchasePlan(ctx, PurchaseSubscriptionPlanInput{
		UserID: registered.User.ID,
		PlanID: plan.ID,
		Now:    time.Date(2026, 7, 9, 0, 0, 0, 0, time.UTC),
	})
	require.NoError(t, err)
	require.NotZero(t, subscription.PurchaseLedgerTransactionID)

	apiKeyBudget := int64(100_000_000)
	apiKeySingleRequestMax := int64(5_000_000)
	apiKeyRow, err := client.APIKey.UpdateOneID(registered.APIKey.ID).
		SetCommercialLimits(&objects.APIKeyCommercialLimits{
			Enabled:                true,
			Currency:               "CNY",
			TotalBudgetMicros:      &apiKeyBudget,
			SingleRequestMaxMicros: &apiKeySingleRequestMax,
		}).
		Save(ctx)
	require.NoError(t, err)
	require.Equal(t, apikey.TypePersonal, apiKeyRow.Type)
	decision, err := admissionSvc.Check(ctx, AdmissionCheckInput{
		Subject:               UserBillingSubject(registered.User.ID),
		ProjectID:             registered.Project.ID,
		ModelID:               "gpt-aggregate",
		APIKey:                apiKeyRow,
		EstimatedChargeMicros: 1_000_000,
	})
	require.NoError(t, err)
	require.True(t, decision.Allowed)
	require.True(t, decision.SubscriptionCovered)
	require.Equal(t, subscription.ID, decision.UserSubscriptionID)

	_, err = client.BillingPriceRule.Create().
		SetScopeType(billingpricerule.ScopeTypeGlobal).
		SetScopeID(0).
		SetModelPattern("*").
		SetPrice(testModelPrice("1")).
		SetReferenceID("final-acceptance-sell-v1").
		Save(ctx)
	require.NoError(t, err)
	channelRow, upstreamAccount := createUsageAggregateUpstreamAccount(t, client, ctx)
	subscriptionUsageLog := createUsageLogForBillingTestWithUpstreamAccount(
		t,
		client,
		ctx,
		registered.Project.ID,
		channelRow.ID,
		upstreamAccount.ID,
		"gpt-aggregate",
		1_000_000,
		500_000,
	)
	subscriptionCoveredRecord, err := processor.BillUsage(ctx, subscriptionUsageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusSkipped, subscriptionCoveredRecord.Status)
	require.Equal(t, subscription.ID, subscriptionCoveredRecord.UserSubscriptionID)
	require.Equal(t, int64(1_500_000), subscriptionCoveredRecord.ChargeAmountMicros)
	require.Equal(t, upstreamAccount.ID, *subscriptionCoveredRecord.UpstreamAccountID)

	walletUsageLog := createUsageLogForBillingTest(t, client, ctx, registered.Project.ID, "wallet-model", 1_000_000, 0)
	walletChargedRecord, err := processor.BillUsage(ctx, walletUsageLog.ID)
	require.NoError(t, err)
	require.Equal(t, usagebillingrecord.StatusCharged, walletChargedRecord.Status)
	require.NotZero(t, walletChargedRecord.LedgerTransactionID)

	account, err = billingAccountSvc.GetBySubject(ctx, UserBillingSubject(registered.User.ID))
	require.NoError(t, err)
	require.Equal(t, int64(23_000_000), account.BalanceMicros)

	rateLimited := 429
	createMonitoringExecution(t, client, ctx, registered.Project.ID, channelRow.ID, upstreamAccount.ID, requestexecution.StatusCompleted, nil, 120, 30, time.Now().UTC().Add(-3*time.Minute))
	createMonitoringExecution(t, client, ctx, registered.Project.ID, channelRow.ID, upstreamAccount.ID, requestexecution.StatusFailed, &rateLimited, 80, 0, time.Now().UTC().Add(-2*time.Minute))
	upstreamSvc := NewUpstreamAccountService(UpstreamAccountServiceParams{Ent: client})
	fromAccountID := upstreamAccount.ID
	toAccountID := upstreamAccount.ID
	err = upstreamSvc.RecordSwitchHistory(ctx, UpstreamAccountSwitchHistoryInput{
		ProjectID:     registered.Project.ID,
		ChannelID:     channelRow.ID,
		FromAccountID: &fromAccountID,
		ToAccountID:   &toAccountID,
		ModelID:       "gpt-aggregate",
		Reason:        "final_acceptance_retry",
		ErrorCode:     &rateLimited,
		ErrorMessage:  "rate limited",
		LatencyMs:     pointerInt64(80),
	})
	require.NoError(t, err)
	summaries, err := upstreamSvc.MonitoringSummaries(ctx, UpstreamAccountMonitoringFilter{AccountID: &upstreamAccount.ID})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.Equal(t, 2, summaries[0].RequestCount)
	require.Equal(t, 1, summaries[0].RateLimitCount)
	require.Equal(t, subscriptionCoveredRecord.ChargeAmountMicros, summaries[0].UserChargeMicros)
	detail, err := upstreamSvc.MonitoringDetail(ctx, upstreamAccount.ID, UpstreamAccountMonitoringFilter{})
	require.NoError(t, err)
	require.Len(t, detail.SwitchHistory, 1)
	require.Equal(t, "final_acceptance_retry", detail.SwitchHistory[0].Reason)

	_, err = operationsSvc.SaveSetting(ctx, SaveCommercialSettingInput{
		Mode:                            commercialsetting.ModeEnforce,
		RequireAdminActionReason:        true,
		PaymentProviderSecretsEncrypted: true,
		WorkersEnabled:                  true,
		OrderExpiryWorkerEnabled:        true,
		HoldExpiryWorkerEnabled:         true,
		SubscriptionExpiryWorkerEnabled: true,
		SubscriptionResetWorkerEnabled:  true,
		FailedBillingRetryWorkerEnabled: true,
		WorkerBatchSize:                 50,
		Currency:                        "CNY",
		Reason:                          "final acceptance",
	})
	require.NoError(t, err)
	firstMaintenance, err := operationsSvc.RunMaintenance(ctx, CommercialMaintenanceRunInput{
		Now:                    time.Date(2026, 7, 9, 1, 0, 0, 0, time.UTC),
		Limit:                  10,
		Reason:                 "final acceptance rebuild",
		RebuildUsageAggregates: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, firstMaintenance.UsageAggregateRecordsProcessed)
	require.NotZero(t, firstMaintenance.UsageAggregateHourlyRows)
	require.NotZero(t, firstMaintenance.UsageAggregateDailyRows)

	secondMaintenance, err := operationsSvc.RunMaintenance(ctx, CommercialMaintenanceRunInput{
		Now:                    time.Date(2026, 7, 9, 1, 5, 0, 0, time.UTC),
		Limit:                  10,
		Reason:                 "final acceptance rebuild again",
		RebuildUsageAggregates: true,
	})
	require.NoError(t, err)
	require.Equal(t, firstMaintenance.UsageAggregateRecordsProcessed, secondMaintenance.UsageAggregateRecordsProcessed)
	require.Equal(t, firstMaintenance.UsageAggregateHourlyRows, secondMaintenance.UsageAggregateHourlyRows)
	require.Equal(t, firstMaintenance.UsageAggregateDailyRows, secondMaintenance.UsageAggregateDailyRows)

	transactions, err := client.LedgerTransaction.Query().All(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, transactions)
	for _, tx := range transactions {
		require.False(t, strings.HasPrefix(tx.IdempotencyKey, "ah-"), "ledger idempotency key leaked full api key")
		require.NotEqual(t, ledgertransaction.Type(""), tx.Type)
	}
}
