package biz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/llm"
)

type UsageBillingProcessorParams struct {
	fx.In

	Ent                   *ent.Client
	PricingService        *PricingService
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
}

type UsageBillingProcessor struct {
	*AbstractService

	pricingService        *PricingService
	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
}

func NewUsageBillingProcessor(params UsageBillingProcessorParams) *UsageBillingProcessor {
	return &UsageBillingProcessor{
		AbstractService:       &AbstractService{db: params.Ent},
		pricingService:        params.PricingService,
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
	}
}

func (p *UsageBillingProcessor) BillUsage(ctx context.Context, usageLogID int) (*ent.UsageBillingRecord, error) {
	existing, err := p.entFromContext(ctx).UsageBillingRecord.Query().
		Where(usagebillingrecord.UsageLogIDEQ(usageLogID)).
		Only(ctx)
	if err == nil {
		return existing, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to query usage billing record: %w", err)
	}

	usageLog, err := p.entFromContext(ctx).UsageLog.Get(ctx, usageLogID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage log: %w", err)
	}

	account, err := p.billingAccountService.GetOrCreateForSubject(ctx, ProjectBillingSubject(usageLog.ProjectID))
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
		record, recordErr := p.createFailedUsageBillingRecord(ctx, failedUsageBillingRecordInput{
			usageLog:       usageLog,
			accountID:      account.ID,
			usageSnapshot:  usageSnapshot,
			idempotencyKey: usageBillingIdempotencyKey(usageLogID),
			err:            err,
		})
		if recordErr != nil {
			return nil, errors.Join(err, recordErr)
		}
		return record, err
	}

	chargeItems, chargeTotal := ComputeUsageCost(usage, priceRule.Price)
	chargeMicros, err := decimalToMicros(chargeTotal)
	if err != nil {
		return nil, err
	}
	costMicros := usageLogCostMicros(usageLog)
	idempotencyKey := usageBillingIdempotencyKey(usageLogID)

	if chargeMicros == 0 {
		record, err := p.entFromContext(ctx).UsageBillingRecord.Create().
			SetUsageLogID(usageLog.ID).
			SetBillingAccountID(account.ID).
			SetProjectID(usageLog.ProjectID).
			SetNillableAPIKeyID(usageLogAPIKeyIDPtr(usageLog)).
			SetModelID(usageLog.ModelID).
			SetUsageSnapshot(objects.JSONRawMessage(usageSnapshot)).
			SetPriceSnapshot(priceRule.Price).
			SetPriceReferenceID(priceRule.ReferenceID).
			SetChargeItems(chargeItems).
			SetCostAmountMicros(costMicros).
			SetChargeAmountMicros(0).
			SetCurrency(priceRule.Currency).
			SetStatus(usagebillingrecord.StatusSkipped).
			SetIdempotencyKey(idempotencyKey).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create skipped usage billing record: %w", err)
		}
		return record, nil
	}

	var record *ent.UsageBillingRecord
	err = p.RunInTransaction(ctx, func(ctx context.Context) error {
		client := p.entFromContext(ctx)

		pending, err := client.UsageBillingRecord.Create().
			SetUsageLogID(usageLog.ID).
			SetBillingAccountID(account.ID).
			SetProjectID(usageLog.ProjectID).
			SetNillableAPIKeyID(usageLogAPIKeyIDPtr(usageLog)).
			SetModelID(usageLog.ModelID).
			SetUsageSnapshot(objects.JSONRawMessage(usageSnapshot)).
			SetPriceSnapshot(priceRule.Price).
			SetPriceReferenceID(priceRule.ReferenceID).
			SetChargeItems(chargeItems).
			SetCostAmountMicros(costMicros).
			SetChargeAmountMicros(chargeMicros).
			SetCurrency(priceRule.Currency).
			SetStatus(usagebillingrecord.StatusPending).
			SetIdempotencyKey(idempotencyKey).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create usage billing record: %w", err)
		}

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

		charged, err := client.UsageBillingRecord.UpdateOneID(pending.ID).
			SetStatus(usagebillingrecord.StatusCharged).
			SetLedgerTransactionID(ledgerTx.ID).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark usage billing record charged: %w", err)
		}

		record = charged
		return nil
	})
	if err != nil {
		failed, recordErr := p.createFailedUsageBillingRecord(ctx, failedUsageBillingRecordInput{
			usageLog:           usageLog,
			accountID:          account.ID,
			usageSnapshot:      usageSnapshot,
			priceSnapshot:      priceRule.Price,
			priceReferenceID:   priceRule.ReferenceID,
			chargeItems:        chargeItems,
			costAmountMicros:   costMicros,
			chargeAmountMicros: chargeMicros,
			currency:           priceRule.Currency,
			idempotencyKey:     idempotencyKey,
			err:                err,
		})
		if recordErr != nil {
			return nil, errors.Join(err, recordErr)
		}
		return failed, err
	}

	return record, nil
}

type failedUsageBillingRecordInput struct {
	usageLog           *ent.UsageLog
	accountID          int
	usageSnapshot      []byte
	priceSnapshot      objects.ModelPrice
	priceReferenceID   string
	chargeItems        []objects.CostItem
	costAmountMicros   int64
	chargeAmountMicros int64
	currency           string
	idempotencyKey     string
	err                error
}

func (p *UsageBillingProcessor) createFailedUsageBillingRecord(ctx context.Context, input failedUsageBillingRecordInput) (*ent.UsageBillingRecord, error) {
	if input.currency == "" {
		input.currency = defaultBillingCurrency
	}

	record, err := p.entFromContext(ctx).UsageBillingRecord.Create().
		SetUsageLogID(input.usageLog.ID).
		SetBillingAccountID(input.accountID).
		SetProjectID(input.usageLog.ProjectID).
		SetNillableAPIKeyID(usageLogAPIKeyIDPtr(input.usageLog)).
		SetModelID(input.usageLog.ModelID).
		SetUsageSnapshot(objects.JSONRawMessage(input.usageSnapshot)).
		SetPriceSnapshot(input.priceSnapshot).
		SetPriceReferenceID(input.priceReferenceID).
		SetChargeItems(input.chargeItems).
		SetCostAmountMicros(input.costAmountMicros).
		SetChargeAmountMicros(input.chargeAmountMicros).
		SetCurrency(input.currency).
		SetStatus(usagebillingrecord.StatusFailed).
		SetIdempotencyKey(input.idempotencyKey).
		SetError(input.err.Error()).
		Save(ctx)
	if ent.IsConstraintError(err) {
		return p.entFromContext(ctx).UsageBillingRecord.Query().
			Where(usagebillingrecord.UsageLogIDEQ(input.usageLog.ID)).
			Only(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create failed usage billing record: %w", err)
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
