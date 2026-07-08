package biz

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingoutbox"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

type BillingOutboxWorkerParams struct {
	fx.In

	Config                BillingConfig
	Ent                   *ent.Client
	UsageBillingProcessor *UsageBillingProcessor
}

type BillingOutboxWorker struct {
	*AbstractService

	config                BillingConfig
	usageBillingProcessor *UsageBillingProcessor
}

func NewBillingOutboxWorker(params BillingOutboxWorkerParams) *BillingOutboxWorker {
	return &BillingOutboxWorker{
		AbstractService:       &AbstractService{db: params.Ent},
		config:                params.Config.normalized(),
		usageBillingProcessor: params.UsageBillingProcessor,
	}
}

func (w *BillingOutboxWorker) RegisterScheduledTasks(ctx context.Context, s *scheduler.Scheduler) error {
	cfg := w.config.normalized()
	if cfg.Mode == AdmissionModeDisabled {
		return nil
	}

	return s.Register(ctx, scheduler.TaskSpec{
		Name:        "billing-outbox",
		Description: "Retry failed commercial billing outbox events",
		FixRate:     time.Duration(cfg.OutboxWorkerIntervalSeconds) * time.Second,
	}, w.runWithSystemContext)
}

func (w *BillingOutboxWorker) runWithSystemContext(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "billing-outbox-worker")
	ctx = ent.NewContext(ctx, w.entFromContext(ctx))

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	processed, err := w.ProcessDueOutbox(ctx)
	if err != nil {
		log.Error(ctx, "billing outbox worker failed", log.Cause(err))
		return
	}
	if processed > 0 {
		log.Info(ctx, "billing outbox worker processed events", log.Int("processed", processed))
	}
}

func (w *BillingOutboxWorker) ProcessDueOutbox(ctx context.Context) (int, error) {
	cfg := w.config.normalized()
	if cfg.Mode == AdmissionModeDisabled {
		return 0, nil
	}

	now := time.Now().UTC()
	events, err := w.entFromContext(ctx).BillingOutbox.Query().
		Where(
			billingoutbox.EventTypeEQ(billingoutbox.EventTypeUsageBillingRequested),
			billingoutbox.StatusIn(billingoutbox.StatusPending, billingoutbox.StatusFailed),
			billingoutbox.AttemptsLT(cfg.OutboxMaxAttempts),
			billingoutbox.Or(
				billingoutbox.NextAttemptAtIsNil(),
				billingoutbox.NextAttemptAtLTE(now),
			),
		).
		Order(billingoutbox.ByID()).
		Limit(cfg.OutboxBatchSize).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("query billing outbox events: %w", err)
	}

	processed := 0
	for _, event := range events {
		ok, err := w.processOne(ctx, event)
		if err != nil {
			log.Warn(ctx, "failed to process billing outbox event",
				log.Int("outbox_id", event.ID),
				log.String("event_key", event.EventKey),
				log.Cause(err),
			)
		}
		if ok {
			processed++
		}
	}

	return processed, nil
}

func (w *BillingOutboxWorker) processOne(ctx context.Context, event *ent.BillingOutbox) (bool, error) {
	claimed, err := w.entFromContext(ctx).BillingOutbox.Update().
		Where(
			billingoutbox.IDEQ(event.ID),
			billingoutbox.StatusIn(billingoutbox.StatusPending, billingoutbox.StatusFailed),
		).
		SetStatus(billingoutbox.StatusProcessing).
		AddAttempts(1).
		SetLastError("").
		ClearNextAttemptAt().
		Save(ctx)
	if err != nil {
		return false, fmt.Errorf("claim billing outbox event: %w", err)
	}
	if claimed == 0 {
		return false, nil
	}

	claimedEvent, err := w.entFromContext(ctx).BillingOutbox.Get(ctx, event.ID)
	if err != nil {
		return true, fmt.Errorf("reload claimed billing outbox event: %w", err)
	}

	if err := w.handleClaimed(ctx, claimedEvent); err != nil {
		nextAttemptAt := time.Now().UTC().Add(time.Duration(w.config.normalized().OutboxRetryDelaySeconds) * time.Second)
		_, updateErr := w.entFromContext(ctx).BillingOutbox.UpdateOneID(claimedEvent.ID).
			SetStatus(billingoutbox.StatusFailed).
			SetLastError(err.Error()).
			SetNextAttemptAt(nextAttemptAt).
			Save(ctx)
		if updateErr != nil {
			return true, fmt.Errorf("%w; mark outbox failed: %v", err, updateErr)
		}

		return true, err
	}

	_, err = w.entFromContext(ctx).BillingOutbox.UpdateOneID(claimedEvent.ID).
		SetStatus(billingoutbox.StatusDone).
		SetLastError("").
		ClearNextAttemptAt().
		Save(ctx)
	if err != nil {
		return true, fmt.Errorf("mark billing outbox event done: %w", err)
	}

	return true, nil
}

func (w *BillingOutboxWorker) handleClaimed(ctx context.Context, event *ent.BillingOutbox) error {
	switch event.EventType {
	case billingoutbox.EventTypeUsageBillingRequested:
		var payload struct {
			UsageLogID int `json:"usage_log_id"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return fmt.Errorf("decode usage billing payload: %w", err)
		}
		if payload.UsageLogID <= 0 {
			return fmt.Errorf("usage billing payload missing usage_log_id")
		}

		_, err := w.usageBillingProcessor.BillUsage(ctx, payload.UsageLogID)
		if err != nil {
			return err
		}

		return nil
	default:
		return fmt.Errorf("unsupported billing outbox event type %q", event.EventType)
	}
}
