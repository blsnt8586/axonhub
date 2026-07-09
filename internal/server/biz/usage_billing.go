package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingoutbox"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

type UsageBillingProcessorParams struct {
	fx.In

	Config                 BillingConfig
	Ent                    *ent.Client
	PricingService         *PricingService
	BillingAccountService  *BillingAccountService
	LedgerService          *LedgerService
	BillingHoldService     *BillingHoldService
	CommercialLimitService *APIKeyCommercialLimitService
	SubscriptionService    *SubscriptionService
}

type UsageBillingProcessor struct {
	*AbstractService

	config                 BillingConfig
	pricingService         *PricingService
	billingAccountService  *BillingAccountService
	ledgerService          *LedgerService
	billingHoldService     *BillingHoldService
	commercialLimitService *APIKeyCommercialLimitService
	subscriptionService    *SubscriptionService
}

func NewUsageBillingProcessor(params UsageBillingProcessorParams) *UsageBillingProcessor {
	return &UsageBillingProcessor{
		AbstractService:        &AbstractService{db: params.Ent},
		config:                 params.Config.normalized(),
		pricingService:         params.PricingService,
		billingAccountService:  params.BillingAccountService,
		ledgerService:          params.LedgerService,
		billingHoldService:     params.BillingHoldService,
		commercialLimitService: params.CommercialLimitService,
		subscriptionService:    params.SubscriptionService,
	}
}

func (p *UsageBillingProcessor) RequestUsageBilling(ctx context.Context, usageLogID int, holdIDs ...int) (*ent.UsageBillingRecord, error) {
	if !p.requestBillingEnabled() {
		return nil, nil
	}

	outbox, err := p.createUsageBillingOutbox(ctx, usageLogID)
	if err != nil {
		return nil, err
	}

	record, err := p.BillUsage(ctx, usageLogID, holdIDs...)
	if err != nil {
		_, updateErr := p.entFromContext(ctx).BillingOutbox.UpdateOneID(outbox.ID).
			SetStatus(billingoutbox.StatusFailed).
			AddAttempts(1).
			SetLastError(err.Error()).
			SetNextAttemptAt(time.Now().UTC().Add(time.Minute)).
			Save(ctx)
		if updateErr != nil {
			return record, errors.Join(err, fmt.Errorf("failed to update usage billing outbox failure: %w", updateErr))
		}

		return record, err
	}

	_, err = p.entFromContext(ctx).BillingOutbox.UpdateOneID(outbox.ID).
		SetStatus(billingoutbox.StatusDone).
		AddAttempts(1).
		SetLastError("").
		ClearNextAttemptAt().
		Save(ctx)
	if err != nil {
		return record, fmt.Errorf("failed to mark usage billing outbox done: %w", err)
	}

	return record, nil
}

func (p *UsageBillingProcessor) requestBillingEnabled() bool {
	cfg := p.config.normalized()
	return cfg.Mode != AdmissionModeDisabled
}

func (p *UsageBillingProcessor) createUsageBillingOutbox(ctx context.Context, usageLogID int) (*ent.BillingOutbox, error) {
	payload, err := json.Marshal(map[string]int{"usage_log_id": usageLogID})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal usage billing outbox payload: %w", err)
	}

	outbox, err := p.entFromContext(ctx).BillingOutbox.Create().
		SetEventKey(usageBillingIdempotencyKey(usageLogID)).
		SetEventType(billingoutbox.EventTypeUsageBillingRequested).
		SetPayload(objects.JSONRawMessage(payload)).
		Save(ctx)
	if ent.IsConstraintError(err) {
		return p.entFromContext(ctx).BillingOutbox.Query().
			Where(billingoutbox.EventKeyEQ(usageBillingIdempotencyKey(usageLogID))).
			Only(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create usage billing outbox: %w", err)
	}

	return outbox, nil
}

func (p *UsageBillingProcessor) BillUsage(ctx context.Context, usageLogID int, holdIDs ...int) (*ent.UsageBillingRecord, error) {
	holdID := 0
	if len(holdIDs) > 0 {
		holdID = holdIDs[0]
	}

	existing, err := p.entFromContext(ctx).UsageBillingRecord.Query().
		Where(usagebillingrecord.UsageLogIDEQ(usageLogID)).
		Only(ctx)
	if err == nil {
		if existing.Status != usagebillingrecord.StatusFailed {
			return existing, nil
		}
		if existing.LedgerTransactionID != 0 {
			return nil, fmt.Errorf("failed usage billing record %d has ledger transaction %d and cannot be retried", existing.ID, existing.LedgerTransactionID)
		}
		if err := p.entFromContext(ctx).UsageBillingRecord.DeleteOneID(existing.ID).Exec(ctx); err != nil {
			return nil, fmt.Errorf("failed to remove failed usage billing record before retry: %w", err)
		}
	} else if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query usage billing record: %w", err)
	}

	usageLog, err := p.entFromContext(ctx).UsageLog.Get(ctx, usageLogID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage log: %w", err)
	}
	if holdID == 0 && p.billingHoldService != nil {
		hold, err := p.billingHoldService.FindHeldHoldForUsageLog(ctx, usageLog)
		if err != nil {
			return nil, err
		}
		if hold != nil {
			holdID = hold.ID
		}
	}

	subject, subjectUserID, ok, err := p.billingSubjectForUsageLog(ctx, usageLog)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	account, err := p.billingAccountService.GetOrCreateForSubject(ctx, subject)
	if err != nil {
		return nil, err
	}

	usage := usageFromUsageLog(usageLog)
	usageSnapshot, err := json.Marshal(usage)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal usage snapshot: %w", err)
	}

	priceRule, err := p.pricingService.FindSellPrice(ctx, usageLog.ProjectID, usageLog.ModelID)
	if err != nil {
		return nil, err
	}

	chargeItems, chargeTotal := ComputeUsageCost(usage, priceRule.Price)
	chargeMicros, err := decimalToMicros(chargeTotal)
	if err != nil {
		return nil, err
	}
	if p.commercialLimitService != nil && usageLog.APIKeyID > 0 {
		apiKey, err := p.entFromContext(ctx).APIKey.Get(ctx, usageLog.APIKeyID)
		if err != nil {
			return nil, fmt.Errorf("failed to get usage log api key for commercial limit check: %w", err)
		}
		_, err = p.commercialLimitService.Check(ctx, APIKeyCommercialLimitCheckInput{
			APIKey:                apiKey,
			Currency:              priceRule.Currency,
			EstimatedChargeMicros: chargeMicros,
		})
		if err != nil {
			return nil, err
		}
	}
	costMicros := usageLogCostMicros(usageLog)
	idempotencyKey := usageBillingIdempotencyKey(usageLogID)

	var record *ent.UsageBillingRecord
	err = p.RunInTransaction(ctx, func(ctx context.Context) error {
		client := p.entFromContext(ctx)

		var coveredSubscription *ent.UserSubscription
		if subject.Type == BillingSubjectTypeUser && p.subscriptionService != nil && chargeMicros > 0 {
			coverage, err := p.subscriptionService.CoverUsage(ctx, SubscriptionCoverageInput{
				UserID:       subject.ID,
				ProjectID:    usageLog.ProjectID,
				ModelID:      usageLog.ModelID,
				AmountMicros: chargeMicros,
			})
			if err != nil {
				return err
			}
			if coverage.Covered {
				coveredSubscription = coverage.Subscription
			}
			if coverage.WalletFallbackDenied {
				if coverage.DenyReason != "" {
					return errors.New(coverage.DenyReason)
				}
				return errors.New("subscription quota exhausted and wallet fallback is disabled")
			}
		}

		create := client.UsageBillingRecord.Create().
			SetUsageLogID(usageLog.ID).
			SetBillingAccountID(account.ID).
			SetProjectID(usageLog.ProjectID).
			SetNillableUserID(subjectUserID).
			SetNillableAPIKeyID(usageLogAPIKeyIDPtr(usageLog)).
			SetModelID(usageLog.ModelID).
			SetUsageSnapshot(objects.JSONRawMessage(usageSnapshot)).
			SetPriceSnapshot(priceRule.Price).
			SetPriceReferenceID(priceRule.ReferenceID).
			SetChargeItems(chargeItems).
			SetCostAmountMicros(costMicros).
			SetChargeAmountMicros(chargeMicros).
			SetCurrency(priceRule.Currency).
			SetIdempotencyKey(idempotencyKey)
		if coveredSubscription != nil {
			create.SetStatus(usagebillingrecord.StatusSkipped).
				SetUserSubscriptionID(coveredSubscription.ID)
		} else if chargeMicros == 0 {
			create.SetStatus(usagebillingrecord.StatusSkipped)
		} else {
			create.SetStatus(usagebillingrecord.StatusPending)
		}

		pending, err := create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create usage billing record: %w", err)
		}

		var ledgerTransactionID int
		if coveredSubscription != nil {
			ledgerTransactionID = 0
		} else if holdID > 0 {
			if p.billingHoldService == nil {
				return fmt.Errorf("billing hold service is required to capture hold %d", holdID)
			}
			hold, err := p.billingHoldService.CaptureHold(ctx, CaptureBillingHoldInput{
				HoldID:        holdID,
				Amount:        chargeTotal,
				Currency:      priceRule.Currency,
				UsageLogID:    &usageLog.ID,
				ReferenceType: "usage_billing_record",
				ReferenceID:   fmt.Sprint(pending.ID),
				Memo:          fmt.Sprintf("usage log %d model %s", usageLog.ID, usageLog.ModelID),
				CreatedByType: ledgertransaction.CreatedByTypeSystem,
				CapturedAt:    time.Now().UTC(),
			})
			if err != nil {
				return err
			}
			ledgerTransactionID = hold.CapturedLedgerTransactionID
		} else if chargeMicros > 0 {
			ledgerTx, err := p.ledgerService.Post(ctx, LedgerPostInput{
				BillingAccountID: account.ID,
				Direction:        ledgertransaction.DirectionDebit,
				Amount:           chargeTotal,
				Currency:         priceRule.Currency,
				Type:             ledgertransaction.TypeUsageCharge,
				IdempotencyKey:   idempotencyKey,
				ReferenceType:    "usage_billing_record",
				ReferenceID:      fmt.Sprint(pending.ID),
				Memo:             fmt.Sprintf("usage log %d model %s", usageLog.ID, usageLog.ModelID),
				CreatedByType:    ledgertransaction.CreatedByTypeSystem,
			})
			if err != nil {
				return err
			}
			ledgerTransactionID = ledgerTx.ID
		}

		update := client.UsageBillingRecord.UpdateOneID(pending.ID)
		if chargeMicros > 0 && coveredSubscription == nil {
			update.SetStatus(usagebillingrecord.StatusCharged)
		}
		if ledgerTransactionID > 0 {
			update.SetLedgerTransactionID(ledgerTransactionID)
		}
		charged, err := update.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark usage billing record charged: %w", err)
		}

		record = charged
		return nil
	})
	if err != nil {
		return nil, err
	}

	return record, nil
}

func usageBillingIdempotencyKey(usageLogID int) string {
	return fmt.Sprintf("usage_billing:%d", usageLogID)
}

func usageLogCostMicros(usageLog *ent.UsageLog) int64 {
	if usageLog.TotalCost == nil {
		return 0
	}

	micros, err := decimalToMicros(decimal.NewFromFloat(*usageLog.TotalCost))
	if err != nil {
		return 0
	}

	return micros
}

func usageLogAPIKeyIDPtr(usageLog *ent.UsageLog) *int {
	if usageLog.APIKeyID <= 0 {
		return nil
	}

	return &usageLog.APIKeyID
}

func (p *UsageBillingProcessor) billingSubjectForUsageLog(ctx context.Context, usageLog *ent.UsageLog) (BillingSubject, *int, bool, error) {
	var apiKey *ent.APIKey
	if usageLog.APIKeyID > 0 {
		loaded, err := p.entFromContext(ctx).APIKey.Get(ctx, usageLog.APIKeyID)
		if err != nil {
			return BillingSubject{}, nil, false, fmt.Errorf("failed to get usage log api key: %w", err)
		}
		apiKey = loaded
	}

	subject, ok := BillingSubjectForAPIKey(p.config, apiKey, usageLog.ProjectID)
	if !ok {
		return BillingSubject{}, nil, false, nil
	}

	if subject.Type != BillingSubjectTypeUser {
		return subject, nil, true, nil
	}

	return subject, &subject.ID, true, nil
}

func usageFromUsageLog(usageLog *ent.UsageLog) *llm.Usage {
	usage := &llm.Usage{
		PromptTokens:     usageLog.PromptTokens,
		CompletionTokens: usageLog.CompletionTokens,
		TotalTokens:      usageLog.TotalTokens,
	}

	if usageLog.PromptAudioTokens != 0 ||
		usageLog.PromptCachedTokens != 0 ||
		usageLog.PromptWriteCachedTokens != 0 ||
		usageLog.PromptWriteCachedTokens5m != 0 ||
		usageLog.PromptWriteCachedTokens1h != 0 {
		usage.PromptTokensDetails = &llm.PromptTokensDetails{
			AudioTokens:            usageLog.PromptAudioTokens,
			CachedTokens:           usageLog.PromptCachedTokens,
			WriteCachedTokens:      usageLog.PromptWriteCachedTokens,
			WriteCached5MinTokens:  usageLog.PromptWriteCachedTokens5m,
			WriteCached1HourTokens: usageLog.PromptWriteCachedTokens1h,
		}
	}

	if usageLog.CompletionAudioTokens != 0 ||
		usageLog.CompletionReasoningTokens != 0 ||
		usageLog.CompletionAcceptedPredictionTokens != 0 ||
		usageLog.CompletionRejectedPredictionTokens != 0 {
		usage.CompletionTokensDetails = &llm.CompletionTokensDetails{
			AudioTokens:              usageLog.CompletionAudioTokens,
			ReasoningTokens:          usageLog.CompletionReasoningTokens,
			AcceptedPredictionTokens: usageLog.CompletionAcceptedPredictionTokens,
			RejectedPredictionTokens: usageLog.CompletionRejectedPredictionTokens,
		}
	}

	return usage
}
