package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/objects"
)

type PaymentServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
	ProviderRegistry      *PaymentProviderRegistry
}

type PaymentService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
	providerRegistry      *PaymentProviderRegistry
}

func NewPaymentService(params PaymentServiceParams) *PaymentService {
	return &PaymentService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
		providerRegistry:      params.ProviderRegistry,
	}
}

const simulatedEPayProviderName = "Simulated ePay"

type CreateRechargeCheckoutInput struct {
	ProjectID          int
	ProviderInstanceID *int
	ProviderType       paymentproviderinstance.ProviderType
	Amount             decimal.Decimal
	Currency           string
	Subject            string
	Metadata           objects.JSONRawMessage
}

func (s *PaymentService) CreateRechargeCheckout(ctx context.Context, input CreateRechargeCheckoutInput) (*PaymentProviderCheckout, error) {
	if s.providerRegistry == nil {
		return nil, fmt.Errorf("payment provider registry is not configured")
	}
	if input.ProjectID <= 0 {
		return nil, fmt.Errorf("project id is required")
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.ProviderType == "" {
		input.ProviderType = paymentproviderinstance.ProviderTypeEpay
	}

	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	provider, err := s.resolvePaymentProvider(ctx, input.ProviderType, input.ProviderInstanceID)
	if err != nil {
		return nil, err
	}
	if provider.Status != paymentproviderinstance.StatusEnabled {
		return nil, fmt.Errorf("payment provider %q is not enabled", provider.Name)
	}
	if provider.Currency != input.Currency {
		return nil, fmt.Errorf("payment currency %s does not match provider currency %s", input.Currency, provider.Currency)
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
		SetProviderInstanceID(provider.ID).
		SetProviderType(paymentorder.ProviderType(provider.ProviderType)).
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

	adapter, err := s.providerRegistry.Adapter(provider.ProviderType)
	if err != nil {
		return nil, err
	}

	checkout, err := adapter.CreateCheckout(ctx, PaymentProviderCheckoutInput{
		ProviderInstance: provider,
		Order:            order,
		Subject:          input.Subject,
	})
	if err != nil {
		return nil, err
	}

	return checkout, nil
}

func (s *PaymentService) GetOrCreateSimulatedEPayProvider(ctx context.Context, publicBaseURL string) (*ent.PaymentProviderInstance, error) {
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:3000"
	}
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")

	cfg := EPayConfig{
		GatewayURL: publicBaseURL + "/payment/simulate/epay/submit",
		PID:        "1001",
		Key:        "axonhub-simulated-epay-secret",
		NotifyURL:  publicBaseURL + "/payment/notify/epay",
		ReturnURL:  publicBaseURL + "/admin",
		Type:       "alipay",
		SiteName:   "AxonHub Local",
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal simulated epay config: %w", err)
	}

	client := s.entFromContext(ctx)
	provider, err := client.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderTypeEQ(paymentproviderinstance.ProviderTypeEpay),
			paymentproviderinstance.NameEQ(simulatedEPayProviderName),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return client.PaymentProviderInstance.Create().
			SetName(simulatedEPayProviderName).
			SetProviderType(paymentproviderinstance.ProviderTypeEpay).
			SetStatus(paymentproviderinstance.StatusEnabled).
			SetCurrency(defaultBillingCurrency).
			SetConfig(objects.JSONRawMessage(raw)).
			Save(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load simulated epay provider: %w", err)
	}

	return client.PaymentProviderInstance.UpdateOneID(provider.ID).
		SetStatus(paymentproviderinstance.StatusEnabled).
		SetCurrency(defaultBillingCurrency).
		SetConfig(objects.JSONRawMessage(raw)).
		Save(ctx)
}

func (s *PaymentService) resolvePaymentProvider(ctx context.Context, providerType paymentproviderinstance.ProviderType, providerInstanceID *int) (*ent.PaymentProviderInstance, error) {
	client := s.entFromContext(ctx)
	if providerInstanceID != nil {
		provider, err := client.PaymentProviderInstance.Get(ctx, *providerInstanceID)
		if err != nil {
			return nil, fmt.Errorf("failed to load payment provider instance: %w", err)
		}
		if provider.ProviderType != providerType {
			return nil, fmt.Errorf("payment provider type mismatch: got %s want %s", provider.ProviderType, providerType)
		}

		return provider, nil
	}

	provider, err := client.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderTypeEQ(providerType),
			paymentproviderinstance.StatusEQ(paymentproviderinstance.StatusEnabled),
		).
		First(ctx)
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("no enabled payment provider found for %s", providerType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load payment provider instance: %w", err)
	}

	return provider, nil
}

type HandleEPayNotifyInput struct {
	Params map[string]string
}

func (s *PaymentService) HandleEPayNotify(ctx context.Context, input HandleEPayNotifyInput) (*ent.PaymentOrder, error) {
	if len(input.Params) == 0 {
		return nil, fmt.Errorf("epay notify params are required")
	}
	orderNo := input.Params["out_trade_no"]
	if orderNo == "" {
		return nil, fmt.Errorf("epay notify missing out_trade_no")
	}
	if input.Params["trade_no"] == "" {
		return nil, fmt.Errorf("epay notify missing trade_no")
	}

	order, err := s.entFromContext(ctx).PaymentOrder.Query().
		Where(paymentorder.OrderNoEQ(orderNo)).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, ErrPaymentOrderNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load payment order: %w", err)
	}
	if order.ProviderType != paymentorder.ProviderTypeEpay {
		return nil, fmt.Errorf("payment order %s is not an epay order", order.OrderNo)
	}
	if order.ProviderInstanceID == nil {
		return nil, fmt.Errorf("payment order %s has no provider instance", order.OrderNo)
	}

	provider, err := s.entFromContext(ctx).PaymentProviderInstance.Get(ctx, *order.ProviderInstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load epay provider: %w", err)
	}
	cfg, err := parseEPayConfig(provider.Config)
	if err != nil {
		return nil, err
	}
	if input.Params["pid"] != cfg.PID {
		return nil, fmt.Errorf("epay pid mismatch")
	}
	if !VerifyEPaySignature(input.Params, cfg.Key) {
		return nil, fmt.Errorf("invalid epay signature")
	}
	if status := input.Params["trade_status"]; status != "TRADE_SUCCESS" {
		return nil, fmt.Errorf("epay trade status is not successful: %s", status)
	}

	notifyAmount, err := decimal.NewFromString(input.Params["money"])
	if err != nil {
		return nil, fmt.Errorf("invalid epay money: %w", err)
	}
	if !notifyAmount.Equal(microsToDecimal(order.AmountMicros)) {
		return nil, fmt.Errorf("epay money mismatch: got %s want %s", notifyAmount, microsToDecimal(order.AmountMicros))
	}

	payload, err := json.Marshal(input.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal epay notify payload: %w", err)
	}

	return s.confirmPaidOrder(ctx, confirmPaidOrderInput{
		OrderNo:            order.OrderNo,
		EventKey:           "epay_notify:" + input.Params["trade_no"],
		ExternalTradeNo:    input.Params["trade_no"],
		Payload:            objects.JSONRawMessage(payload),
		ProviderType:       paymentevent.ProviderTypeEpay,
		EventType:          "epay_notify",
		CreatedByType:      ledgertransaction.CreatedByTypeProvider,
		CreatedByID:        fmt.Sprint(provider.ID),
		LedgerMemoProvider: "epay",
	})
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

	return s.confirmPaidOrder(ctx, confirmPaidOrderInput{
		OrderNo:            input.OrderNo,
		EventKey:           input.EventKey,
		ExternalTradeNo:    input.ExternalTradeNo,
		Payload:            input.Payload,
		PaidAt:             input.PaidAt,
		ProviderType:       paymentevent.ProviderTypeManual,
		EventType:          "manual_paid",
		CreatedByType:      ledgertransaction.CreatedByTypeAdmin,
		CreatedByID:        input.ActorID,
		LedgerMemoProvider: "manual recharge",
	})
}

type confirmPaidOrderInput struct {
	OrderNo            string
	EventKey           string
	ExternalTradeNo    string
	Payload            objects.JSONRawMessage
	PaidAt             *time.Time
	ProviderType       paymentevent.ProviderType
	EventType          string
	CreatedByType      ledgertransaction.CreatedByType
	CreatedByID        string
	LedgerMemoProvider string
}

func (s *PaymentService) confirmPaidOrder(ctx context.Context, input confirmPaidOrderInput) (*ent.PaymentOrder, error) {
	if input.OrderNo == "" {
		return nil, fmt.Errorf("order no is required")
	}
	if input.EventKey == "" {
		input.EventKey = string(input.ProviderType) + "_paid:" + input.OrderNo
	}
	if input.EventType == "" {
		input.EventType = string(input.ProviderType) + "_paid"
	}
	if input.PaidAt == nil {
		now := time.Now().UTC()
		input.PaidAt = &now
	}
	if input.LedgerMemoProvider == "" {
		input.LedgerMemoProvider = string(input.ProviderType)
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
			SetNillableProviderInstanceID(order.ProviderInstanceID).
			SetProviderType(input.ProviderType).
			SetEventType(input.EventType).
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
			Memo:             fmt.Sprintf("%s order %s", input.LedgerMemoProvider, order.OrderNo),
			CreatedByType:    input.CreatedByType,
			CreatedByID:      input.CreatedByID,
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
