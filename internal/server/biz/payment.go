package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/objects"
)

type PaymentServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
}

type PaymentService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
}

func NewPaymentService(params PaymentServiceParams) *PaymentService {
	return &PaymentService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
	}
}

type CreateManualRechargeOrderInput struct {
	ProjectID int
	Amount    decimal.Decimal
	Currency  string
	Metadata  objects.JSONRawMessage
}

func (s *PaymentService) CreateManualRechargeOrder(ctx context.Context, input CreateManualRechargeOrderInput) (*ent.PaymentOrder, error) {
	if input.ProjectID <= 0 {
		return nil, fmt.Errorf("project id is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}

	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	account, err := s.billingAccountService.GetOrCreateForSubject(ctx, ProjectBillingSubject(input.ProjectID))
	if err != nil {
		return nil, err
	}
	if account.Currency != input.Currency {
		return nil, fmt.Errorf("payment currency %s does not match account currency %s", input.Currency, account.Currency)
	}

	orderNo, err := newPaymentOrderNo()
	if err != nil {
		return nil, err
	}

	create := s.entFromContext(ctx).PaymentOrder.Create().
		SetOrderNo(orderNo).
		SetProjectID(input.ProjectID).
		SetBillingAccountID(account.ID).
		SetProviderType(paymentorder.ProviderTypeManual).
		SetPurpose(paymentorder.PurposeRecharge).
		SetAmountMicros(amountMicros).
		SetCurrency(input.Currency).
		SetStatus(paymentorder.StatusPending)
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}

	order, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment order: %w", err)
	}

	return order, nil
}

type ConfirmManualPaymentInput struct {
	OrderNo         string
	EventKey        string
	ExternalTradeNo string
	Payload         objects.JSONRawMessage
	PaidAt          *time.Time
	ActorID         string
}

func (s *PaymentService) ConfirmManualPayment(ctx context.Context, input ConfirmManualPaymentInput) (*ent.PaymentOrder, error) {
	if input.OrderNo == "" {
		return nil, fmt.Errorf("order no is required")
	}
	if input.EventKey == "" {
		input.EventKey = "manual_paid:" + input.OrderNo
	}
	if input.PaidAt == nil {
		now := time.Now().UTC()
		input.PaidAt = &now
	}

	var paidOrder *ent.PaymentOrder
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		order, err := client.PaymentOrder.Query().
			Where(paymentorder.OrderNoEQ(input.OrderNo)).
			Only(ctx)
		if ent.IsNotFound(err) {
			return ErrPaymentOrderNotFound
		}
		if err != nil {
			return fmt.Errorf("failed to load payment order: %w", err)
		}

		if order.Status == paymentorder.StatusPaid {
			paidOrder = order
			return nil
		}
		if order.Status != paymentorder.StatusPending {
			return fmt.Errorf("%w: %s", ErrPaymentOrderNotPayable, order.Status)
		}

		event, err := client.PaymentEvent.Create().
			SetEventKey(input.EventKey).
			SetPaymentOrderID(order.ID).
			SetProviderType(paymentevent.ProviderTypeManual).
			SetEventType("manual_paid").
			SetPayload(input.Payload).
			SetStatus(paymentevent.StatusReceived).
			Save(ctx)
		if ent.IsConstraintError(err) {
			event, err = client.PaymentEvent.Query().
				Where(paymentevent.EventKeyEQ(input.EventKey)).
				Only(ctx)
		}
		if err != nil {
			return fmt.Errorf("failed to create payment event: %w", err)
		}
		if event.PaymentOrderID == nil || *event.PaymentOrderID != order.ID {
			return fmt.Errorf("payment event %q does not belong to order %s", input.EventKey, input.OrderNo)
		}
		if event.Status == paymentevent.StatusProcessed {
			paidOrder = order
			return nil
		}

		ledgerTx, err := s.ledgerService.Post(ctx, LedgerPostInput{
			BillingAccountID: order.BillingAccountID,
			Direction:        ledgertransaction.DirectionCredit,
			Amount:           microsToDecimal(order.AmountMicros),
			Currency:         order.Currency,
			Type:             ledgertransaction.TypePaymentRecharge,
			IdempotencyKey:   paymentLedgerIdempotencyKey(order.ID),
			ReferenceType:    "payment_order",
			ReferenceID:      fmt.Sprint(order.ID),
			Memo:             fmt.Sprintf("manual recharge order %s", order.OrderNo),
			CreatedByType:    ledgertransaction.CreatedByTypeAdmin,
			CreatedByID:      input.ActorID,
		})
		if err != nil {
			_, _ = client.PaymentEvent.UpdateOneID(event.ID).
				SetStatus(paymentevent.StatusFailed).
				SetError(err.Error()).
				Save(ctx)
			return err
		}

		update := client.PaymentOrder.UpdateOneID(order.ID).
			SetStatus(paymentorder.StatusPaid).
			SetPaidAt(*input.PaidAt).
			SetLedgerTransactionID(ledgerTx.ID)
		if input.ExternalTradeNo != "" {
			update.SetExternalTradeNo(input.ExternalTradeNo)
		}

		paidOrder, err = update.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark payment order paid: %w", err)
		}

		_, err = client.PaymentEvent.UpdateOneID(event.ID).
			SetStatus(paymentevent.StatusProcessed).
			SetError("").
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark payment event processed: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paidOrder, nil
}

func paymentLedgerIdempotencyKey(orderID int) string {
	return fmt.Sprintf("payment_order:%d:paid", orderID)
}

func newPaymentOrderNo() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate payment order no: %w", err)
	}

	return fmt.Sprintf("pay_%d_%s", time.Now().UTC().UnixNano(), hex.EncodeToString(b[:])), nil
}
