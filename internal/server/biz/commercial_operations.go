package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/commercialsetting"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
)

const defaultCommercialSettingKey = "default"

type CommercialOperationsServiceParams struct {
	fx.In

	Ent                   *ent.Client
	PaymentService        *PaymentService
	BillingHoldService    *BillingHoldService
	SubscriptionService   *SubscriptionService
	AffiliateService      *AffiliateService
	BillingOutboxWorker   *BillingOutboxWorker
	UsageAggregateService *UsageAggregateService
	BillingAuditService   *BillingAuditService `optional:"true"`
}

type CommercialOperationsService struct {
	*AbstractService

	paymentService        *PaymentService
	billingHoldService    *BillingHoldService
	subscriptionService   *SubscriptionService
	affiliateService      *AffiliateService
	billingOutboxWorker   *BillingOutboxWorker
	usageAggregateService *UsageAggregateService
	auditService          *BillingAuditService
}

type SaveCommercialSettingInput struct {
	Mode                             commercialsetting.Mode
	RequireAdminActionReason         bool
	PaymentProviderSecretsEncrypted  bool
	WorkersEnabled                   bool
	OrderExpiryWorkerEnabled         bool
	HoldExpiryWorkerEnabled          bool
	SubscriptionExpiryWorkerEnabled  bool
	SubscriptionResetWorkerEnabled   bool
	AffiliateRebateThawWorkerEnabled bool
	FailedBillingRetryWorkerEnabled  bool
	WorkerBatchSize                  int
	Currency                         string
	Reason                           string
}

type CommercialMaintenanceRunInput struct {
	Now                    time.Time
	Limit                  int
	Reason                 string
	RebuildUsageAggregates bool
}

type CommercialMaintenanceRunResult struct {
	OrderExpiryProcessed           int `json:"orderExpiryProcessed"`
	HoldExpiryProcessed            int `json:"holdExpiryProcessed"`
	SubscriptionExpiryProcessed    int `json:"subscriptionExpiryProcessed"`
	SubscriptionResetProcessed     int `json:"subscriptionResetProcessed"`
	AffiliateRebateThawProcessed   int `json:"affiliateRebateThawProcessed"`
	FailedBillingRetryProcessed    int `json:"failedBillingRetryProcessed"`
	UsageAggregateRecordsProcessed int `json:"usageAggregateRecordsProcessed"`
	UsageAggregateHourlyRows       int `json:"usageAggregateHourlyRows"`
	UsageAggregateDailyRows        int `json:"usageAggregateDailyRows"`
}

func NewCommercialOperationsService(params CommercialOperationsServiceParams) *CommercialOperationsService {
	return &CommercialOperationsService{
		AbstractService:       &AbstractService{db: params.Ent},
		paymentService:        params.PaymentService,
		billingHoldService:    params.BillingHoldService,
		subscriptionService:   params.SubscriptionService,
		affiliateService:      params.AffiliateService,
		billingOutboxWorker:   params.BillingOutboxWorker,
		usageAggregateService: params.UsageAggregateService,
		auditService:          params.BillingAuditService,
	}
}

func (s *CommercialOperationsService) GetOrCreateSetting(ctx context.Context) (*ent.CommercialSetting, error) {
	setting, err := s.entFromContext(ctx).CommercialSetting.Query().
		Where(commercialsetting.KeyEQ(defaultCommercialSettingKey)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return s.entFromContext(ctx).CommercialSetting.Create().
			SetKey(defaultCommercialSettingKey).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load commercial setting: %w", err)
	}
	return setting, nil
}

func (s *CommercialOperationsService) EnsureDefaults(ctx context.Context) error {
	ctx = authz.WithSystemBypass(ctx, "commercial-operations-ensure-defaults")
	ctx = ent.NewContext(ctx, s.entFromContext(ctx))
	if _, err := s.GetOrCreateSetting(ctx); err != nil {
		return err
	}

	migratedProviders := make([]*ent.PaymentProviderInstance, 0)
	if err := s.RunInTransaction(ctx, func(txCtx context.Context) error {
		providers, err := s.entFromContext(txCtx).PaymentProviderInstance.Query().
			Where(paymentproviderinstance.ProviderTypeEQ(paymentproviderinstance.ProviderTypeEpay)).
			Order(ent.Asc(paymentproviderinstance.FieldID)).
			All(txCtx)
		if err != nil {
			return fmt.Errorf("failed to check payment provider config migration safety: %w", err)
		}
		for _, provider := range providers {
			migrated, err := s.ensureEPayProviderSecretEncrypted(txCtx, provider)
			if err != nil {
				return fmt.Errorf("payment provider %q has invalid epay config: %w", provider.Name, err)
			}
			if migrated {
				migratedProviders = append(migratedProviders, provider)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, provider := range migratedProviders {
		log.Info(ctx, "encrypted legacy payment provider secret",
			log.Int("payment_provider_id", provider.ID),
			log.String("payment_provider_name", provider.Name))
	}
	return nil
}

func (s *CommercialOperationsService) ensureEPayProviderSecretEncrypted(ctx context.Context, provider *ent.PaymentProviderInstance) (bool, error) {
	var stored EPayConfig
	if err := json.Unmarshal(provider.Config, &stored); err != nil {
		return false, fmt.Errorf("failed to decode stored epay config: %w", err)
	}

	parsed, err := ParseEPayConfig(provider.Config)
	if err != nil {
		return false, err
	}
	if strings.HasPrefix(stored.Key, encryptedPaymentSecretPrefix) {
		return false, nil
	}

	encrypted, err := encryptEPayConfig(*parsed)
	if err != nil {
		return false, fmt.Errorf("failed to encrypt legacy epay key: %w", err)
	}
	raw, err := json.Marshal(encrypted)
	if err != nil {
		return false, fmt.Errorf("failed to encode encrypted epay config: %w", err)
	}

	if _, err := s.entFromContext(ctx).PaymentProviderInstance.UpdateOneID(provider.ID).
		SetConfig(objects.JSONRawMessage(raw)).
		Save(ctx); err != nil {
		return false, fmt.Errorf("failed to persist encrypted epay config: %w", err)
	}

	return true, nil
}

func (s *CommercialOperationsService) SaveSetting(ctx context.Context, input SaveCommercialSettingInput) (*ent.CommercialSetting, error) {
	if input.Mode == "" {
		input.Mode = commercialsetting.ModeEnforce
	}
	switch input.Mode {
	case commercialsetting.ModeDisabled, commercialsetting.ModeWarn, commercialsetting.ModeEnforce:
	default:
		return nil, fmt.Errorf("unsupported commercial mode %q", input.Mode)
	}
	if input.WorkerBatchSize <= 0 {
		input.WorkerBatchSize = 100
	}
	if input.WorkerBatchSize > 1000 {
		return nil, fmt.Errorf("worker batch size cannot exceed 1000")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}

	current, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return nil, err
	}
	updated, err := s.entFromContext(ctx).CommercialSetting.UpdateOneID(current.ID).
		SetMode(input.Mode).
		SetRequireAdminActionReason(input.RequireAdminActionReason).
		SetPaymentProviderSecretsEncrypted(input.PaymentProviderSecretsEncrypted).
		SetWorkersEnabled(input.WorkersEnabled).
		SetOrderExpiryWorkerEnabled(input.OrderExpiryWorkerEnabled).
		SetHoldExpiryWorkerEnabled(input.HoldExpiryWorkerEnabled).
		SetSubscriptionExpiryWorkerEnabled(input.SubscriptionExpiryWorkerEnabled).
		SetSubscriptionResetWorkerEnabled(input.SubscriptionResetWorkerEnabled).
		SetAffiliateRebateThawWorkerEnabled(input.AffiliateRebateThawWorkerEnabled).
		SetFailedBillingRetryWorkerEnabled(input.FailedBillingRetryWorkerEnabled).
		SetWorkerBatchSize(input.WorkerBatchSize).
		SetCurrency(input.Currency).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to save commercial setting: %w", err)
	}
	if s.auditService != nil {
		if _, auditErr := s.auditService.RecordAdmin(ctx, BillingAuditInput{
			Action:     "commercial_setting.save",
			TargetType: "commercial_setting",
			TargetID:   AuditTargetID(updated.ID),
			Reason:     input.Reason,
		}); auditErr != nil {
			log.Warn(ctx, "failed to record commercial setting audit", log.Cause(auditErr))
		}
	}
	return updated, nil
}

func (s *CommercialOperationsService) RunMaintenance(ctx context.Context, input CommercialMaintenanceRunInput) (CommercialMaintenanceRunResult, error) {
	if input.Now.IsZero() {
		input.Now = time.Now().UTC()
	}
	setting, err := s.GetOrCreateSetting(ctx)
	if err != nil {
		return CommercialMaintenanceRunResult{}, err
	}
	limit := input.Limit
	if limit <= 0 {
		limit = setting.WorkerBatchSize
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		return CommercialMaintenanceRunResult{}, fmt.Errorf("maintenance limit cannot exceed 1000")
	}
	if !setting.WorkersEnabled {
		s.recordMaintenanceAudit(ctx, input.Reason)
		return CommercialMaintenanceRunResult{}, nil
	}

	var result CommercialMaintenanceRunResult
	runCtx := authz.WithSystemBypass(ctx, "commercial-maintenance")
	runCtx = ent.NewContext(runCtx, s.entFromContext(ctx))

	if setting.OrderExpiryWorkerEnabled && s.paymentService != nil {
		result.OrderExpiryProcessed, err = s.paymentService.ExpirePendingPaymentOrders(runCtx, input.Now, limit)
		if err != nil {
			return result, err
		}
	}
	if setting.HoldExpiryWorkerEnabled && s.billingHoldService != nil {
		result.HoldExpiryProcessed, err = s.billingHoldService.ExpireDueHolds(runCtx, input.Now, limit)
		if err != nil {
			return result, err
		}
	}
	if setting.SubscriptionExpiryWorkerEnabled && s.subscriptionService != nil {
		result.SubscriptionExpiryProcessed, err = s.subscriptionService.ExpireDue(runCtx, input.Now, limit)
		if err != nil {
			return result, err
		}
	}
	if setting.SubscriptionResetWorkerEnabled && s.subscriptionService != nil {
		result.SubscriptionResetProcessed, err = s.subscriptionService.ResetDueUsageWindows(runCtx, input.Now, limit)
		if err != nil {
			return result, err
		}
	}
	if setting.AffiliateRebateThawWorkerEnabled && s.affiliateService != nil {
		result.AffiliateRebateThawProcessed, err = s.affiliateService.ThawDueRebates(runCtx, input.Now, limit)
		if err != nil {
			return result, err
		}
	}
	if setting.FailedBillingRetryWorkerEnabled && s.billingOutboxWorker != nil {
		result.FailedBillingRetryProcessed, err = s.billingOutboxWorker.ProcessDueOutbox(runCtx)
		if err != nil {
			return result, err
		}
	}
	if input.RebuildUsageAggregates && s.usageAggregateService != nil {
		rebuild, err := s.usageAggregateService.Rebuild(runCtx, UsageAggregateRebuildInput{})
		if err != nil {
			return result, err
		}
		result.UsageAggregateRecordsProcessed = rebuild.RecordsProcessed
		result.UsageAggregateHourlyRows = rebuild.HourlyRows
		result.UsageAggregateDailyRows = rebuild.DailyRows
	}

	s.recordMaintenanceAudit(ctx, input.Reason)
	return result, nil
}

func (s *CommercialOperationsService) recordMaintenanceAudit(ctx context.Context, reason string) {
	if s.auditService == nil {
		return
	}
	if _, auditErr := s.auditService.RecordAdmin(ctx, BillingAuditInput{
		Action:     "commercial_maintenance.run",
		TargetType: "commercial_maintenance",
		Reason:     reason,
	}); auditErr != nil {
		log.Warn(ctx, "failed to record commercial maintenance audit", log.Cause(auditErr))
	}
}
