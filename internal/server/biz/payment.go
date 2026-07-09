package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/paymentevent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/ent/promousage"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/scheduler"
)

type PaymentServiceParams struct {
	fx.In

	Ent                   *ent.Client
	BillingAccountService *BillingAccountService
	LedgerService         *LedgerService
	ProviderRegistry      *PaymentProviderRegistry
	PromoCodeService      *PromoCodeService           `optional:"true"`
	AffiliateService      *AffiliateService           `optional:"true"`
	NotificationService   *BillingNotificationService `optional:"true"`
}

type PaymentService struct {
	*AbstractService

	billingAccountService *BillingAccountService
	ledgerService         *LedgerService
	providerRegistry      *PaymentProviderRegistry
	promoCodeService      *PromoCodeService
	affiliateService      *AffiliateService
	notificationService   *BillingNotificationService
}

func NewPaymentService(params PaymentServiceParams) *PaymentService {
	return &PaymentService{
		AbstractService:       &AbstractService{db: params.Ent},
		billingAccountService: params.BillingAccountService,
		ledgerService:         params.LedgerService,
		providerRegistry:      params.ProviderRegistry,
		promoCodeService:      params.PromoCodeService,
		affiliateService:      params.AffiliateService,
		notificationService:   params.NotificationService,
	}
}

func (s *PaymentService) RegisterScheduledTasks(ctx context.Context, sched *scheduler.Scheduler) error {
	return sched.Register(ctx, scheduler.TaskSpec{
		Name:        "payment-order-expiry",
		Description: "Expire pending payment orders after their payment window closes",
		FixRate:     time.Minute,
	}, s.runExpiryWithSystemContext)
}

func (s *PaymentService) runExpiryWithSystemContext(ctx context.Context) {
	ctx = authz.WithSystemBypass(ctx, "payment-order-expiry-worker")
	ctx = ent.NewContext(ctx, s.entFromContext(ctx))

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	expired, err := s.ExpirePendingPaymentOrders(ctx, time.Now().UTC(), 100)
	if err != nil {
		log.Error(ctx, "payment order expiry worker failed", log.Cause(err))
		return
	}
	if expired > 0 {
		log.Info(ctx, "payment order expiry worker expired orders", log.Int("expired", expired))
	}
}

const simulatedEPayProviderName = "Simulated ePay"
const defaultPaymentOrderTTL = 30 * time.Minute

type UpsertEPayProviderInput struct {
	Name       string
	Status     paymentproviderinstance.Status
	Currency   string
	GatewayURL string
	PID        string
	Key        string
	NotifyURL  string
	ReturnURL  string
	Type       string
	SiteName   string
}

func (s *PaymentService) UpsertEPayProvider(ctx context.Context, input UpsertEPayProviderInput) (*ent.PaymentProviderInstance, error) {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return nil, fmt.Errorf("payment provider name is required")
	}
	if input.Name == simulatedEPayProviderName {
		return nil, fmt.Errorf("payment provider name %q is reserved for local simulation", simulatedEPayProviderName)
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.Status == "" {
		input.Status = paymentproviderinstance.StatusEnabled
	}
	if input.Status != paymentproviderinstance.StatusEnabled && input.Status != paymentproviderinstance.StatusDisabled {
		return nil, fmt.Errorf("unsupported payment provider status %q", input.Status)
	}

	cfg := EPayConfig{
		GatewayURL: strings.TrimSpace(input.GatewayURL),
		PID:        strings.TrimSpace(input.PID),
		Key:        strings.TrimSpace(input.Key),
		NotifyURL:  strings.TrimSpace(input.NotifyURL),
		ReturnURL:  strings.TrimSpace(input.ReturnURL),
		Type:       strings.TrimSpace(input.Type),
		SiteName:   strings.TrimSpace(input.SiteName),
	}
	if cfg.Type == "" {
		cfg.Type = "alipay"
	}
	if cfg.SiteName == "" {
		cfg.SiteName = "AxonHub"
	}

	client := s.entFromContext(ctx)
	existing, err := client.PaymentProviderInstance.Query().
		Where(
			paymentproviderinstance.ProviderTypeEQ(paymentproviderinstance.ProviderTypeEpay),
			paymentproviderinstance.NameEQ(input.Name),
		).
		Only(ctx)
	if err != nil && !ent.IsNotFound(err) {
		return nil, fmt.Errorf("failed to load epay provider: %w", err)
	}
	if existing != nil && cfg.Key == "" {
		existingCfg, err := parseEPayConfig(existing.Config)
		if err != nil {
			return nil, fmt.Errorf("failed to reuse existing epay key: %w", err)
		}
		cfg.Key = existingCfg.Key
	}

	if err := validateEPayConfig(cfg); err != nil {
		return nil, err
	}
	cfg, err = encryptEPayConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt epay config: %w", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal epay config: %w", err)
	}

	if existing == nil {
		return client.PaymentProviderInstance.Create().
			SetName(input.Name).
			SetProviderType(paymentproviderinstance.ProviderTypeEpay).
			SetStatus(input.Status).
			SetCurrency(input.Currency).
			SetConfig(objects.JSONRawMessage(raw)).
			Save(ctx)
	}

	return client.PaymentProviderInstance.UpdateOneID(existing.ID).
		SetStatus(input.Status).
		SetCurrency(input.Currency).
		SetConfig(objects.JSONRawMessage(raw)).
		Save(ctx)
}

func validateEPayConfig(cfg EPayConfig) error {
	if cfg.GatewayURL == "" {
		return fmt.Errorf("epay gateway_url is required")
	}
	if cfg.PID == "" {
		return fmt.Errorf("epay pid is required")
	}
	if cfg.Key == "" {
		return fmt.Errorf("epay key is required")
	}
	if cfg.NotifyURL == "" {
		return fmt.Errorf("epay notify_url is required")
	}
	if cfg.ReturnURL == "" {
		return fmt.Errorf("epay return_url is required")
	}
	if err := validateHTTPURL("epay gateway_url", cfg.GatewayURL); err != nil {
		return err
	}
	if err := validateHTTPURL("epay notify_url", cfg.NotifyURL); err != nil {
		return err
	}
	if err := validateHTTPURL("epay return_url", cfg.ReturnURL); err != nil {
		return err
	}

	return nil
}

func validateHTTPURL(fieldName string, rawURL string) error {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return fmt.Errorf("%s is invalid: %w", fieldName, err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%s must use http or https", fieldName)
	}
	if parsed.Host == "" {
		return fmt.Errorf("%s must include host", fieldName)
	}

	return nil
}

type CreateRechargeCheckoutInput struct {
	ProjectID          int
	BillingSubject     BillingSubject
	ProviderInstanceID *int
	ProviderType       paymentproviderinstance.ProviderType
	Amount             decimal.Decimal
	Currency           string
	Subject            string
	Metadata           objects.JSONRawMessage
	PromoCode          string
}

func (s *PaymentService) CreateRechargeCheckout(ctx context.Context, input CreateRechargeCheckoutInput) (*PaymentProviderCheckout, error) {
	if s.providerRegistry == nil {
		return nil, fmt.Errorf("payment provider registry is not configured")
	}
	if input.BillingSubject.Type == "" {
		if input.ProjectID <= 0 {
			return nil, fmt.Errorf("billing subject is required")
		}
		input.BillingSubject = ProjectBillingSubject(input.ProjectID)
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

	account, err := s.billingAccountService.GetOrCreateForSubject(ctx, input.BillingSubject)
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

	payableAmountMicros := amountMicros
	discountAmountMicros := int64(0)
	var promoApplication *PromoApplication

	var order *ent.PaymentOrder
	err = s.RunInTransaction(ctx, func(ctx context.Context) error {
		if strings.TrimSpace(input.PromoCode) != "" {
			if input.BillingSubject.Type != BillingSubjectTypeUser {
				return fmt.Errorf("promo code recharge is only supported for user billing accounts")
			}
			if s.promoCodeService == nil {
				return fmt.Errorf("promo code service is not configured")
			}

			application, err := s.promoCodeService.Apply(ctx, PromoApplyInput{
				PromoQuoteInput: PromoQuoteInput{
					Code:                 input.PromoCode,
					Scope:                promousage.ScopeRecharge,
					UserID:               input.BillingSubject.ID,
					OriginalAmountMicros: amountMicros,
					Currency:             input.Currency,
				},
				BillingAccountID: account.ID,
				IdempotencyKey:   "promo:recharge:" + orderNo,
				Status:           promousage.StatusReserved,
			})
			if err != nil {
				return err
			}
			promoApplication = application
			payableAmountMicros = application.PayableAmountMicros
			discountAmountMicros = application.DiscountAmountMicros
		}

		create := s.entFromContext(ctx).PaymentOrder.Create().
			SetOrderNo(orderNo).
			SetProjectID(input.ProjectID).
			SetBillingAccountID(account.ID).
			SetProviderInstanceID(provider.ID).
			SetProviderType(paymentorder.ProviderType(provider.ProviderType)).
			SetPurpose(paymentorder.PurposeRecharge).
			SetAmountMicros(amountMicros).
			SetPayableAmountMicros(payableAmountMicros).
			SetDiscountAmountMicros(discountAmountMicros).
			SetCurrency(input.Currency).
			SetStatus(paymentorder.StatusPending).
			SetExpiresAt(time.Now().UTC().Add(defaultPaymentOrderTTL))
		if len(input.Metadata) > 0 {
			create.SetMetadata(input.Metadata)
		}
		if promoApplication != nil && promoApplication.Code != nil {
			create.SetPromoCodeID(promoApplication.Code.ID)
		}

		var err error
		order, err = create.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create payment order: %w", err)
		}
		if promoApplication != nil && promoApplication.Usage != nil {
			if _, err := s.entFromContext(ctx).PromoUsage.UpdateOneID(promoApplication.Usage.ID).SetPaymentOrderID(order.ID).Save(ctx); err != nil {
				return fmt.Errorf("failed to attach promo usage to payment order: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
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
	cfg, err = encryptEPayConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt simulated epay config: %w", err)
	}
	raw, err = json.Marshal(cfg)
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

type HandleEPayReturnInput struct {
	Params map[string]string
}

type EPayReturnStatus struct {
	Order       *ent.PaymentOrder
	TradeNo     string
	TradeStatus string
	Paid        bool
}

func (s *PaymentService) HandleEPayReturn(ctx context.Context, input HandleEPayReturnInput) (*EPayReturnStatus, error) {
	if len(input.Params) == 0 {
		return nil, fmt.Errorf("epay return params are required")
	}
	orderNo := input.Params["out_trade_no"]
	if orderNo == "" {
		return nil, fmt.Errorf("epay return missing out_trade_no")
	}
	tradeNo := input.Params["trade_no"]
	if tradeNo == "" {
		return nil, fmt.Errorf("epay return missing trade_no")
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

	status := input.Params["trade_status"]
	if status != "TRADE_SUCCESS" {
		return nil, fmt.Errorf("epay trade status is not successful: %s", status)
	}
	notifyAmount, err := decimal.NewFromString(input.Params["money"])
	if err != nil {
		return nil, fmt.Errorf("invalid epay money: %w", err)
	}
	expectedAmount := microsToDecimal(paymentOrderPayableAmountMicros(order))
	if !notifyAmount.Equal(expectedAmount) {
		return nil, fmt.Errorf("epay money mismatch: got %s want %s", notifyAmount, expectedAmount)
	}

	return &EPayReturnStatus{
		Order:       order,
		TradeNo:     tradeNo,
		TradeStatus: status,
		Paid:        order.Status == paymentorder.StatusPaid,
	}, nil
}

type HandleEPayNotifyInput struct {
	Params map[string]string
}

func (s *PaymentService) HandleEPayNotify(ctx context.Context, input HandleEPayNotifyInput) (*ent.PaymentOrder, error) {
	if len(input.Params) == 0 {
		return nil, fmt.Errorf("epay notify params are required")
	}
	fail := func(reason string, order *ent.PaymentOrder, provider *ent.PaymentProviderInstance, err error) (*ent.PaymentOrder, error) {
		if recordErr := s.recordFailedEPayNotify(ctx, recordFailedEPayNotifyInput{
			Params:   input.Params,
			Order:    order,
			Provider: provider,
			Reason:   reason,
			Err:      err,
		}); recordErr != nil {
			return nil, fmt.Errorf("%w; failed to record epay notify failure: %v", err, recordErr)
		}
		return nil, err
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
		return fail("unknown_order", nil, nil, ErrPaymentOrderNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load payment order: %w", err)
	}
	if order.ProviderType != paymentorder.ProviderTypeEpay {
		return fail("provider_type_mismatch", order, nil, fmt.Errorf("payment order %s is not an epay order", order.OrderNo))
	}
	if order.ProviderInstanceID == nil {
		return fail("missing_provider_instance", order, nil, fmt.Errorf("payment order %s has no provider instance", order.OrderNo))
	}

	provider, err := s.entFromContext(ctx).PaymentProviderInstance.Get(ctx, *order.ProviderInstanceID)
	if err != nil {
		return fail("provider_load_failed", order, nil, fmt.Errorf("failed to load epay provider: %w", err))
	}
	cfg, err := parseEPayConfig(provider.Config)
	if err != nil {
		return fail("provider_config_invalid", order, provider, err)
	}
	if input.Params["pid"] != cfg.PID {
		return fail("pid_mismatch", order, provider, fmt.Errorf("epay pid mismatch"))
	}
	if !VerifyEPaySignature(input.Params, cfg.Key) {
		return fail("invalid_signature", order, provider, fmt.Errorf("invalid epay signature"))
	}
	if status := input.Params["trade_status"]; status != "TRADE_SUCCESS" {
		return fail("trade_not_success", order, provider, fmt.Errorf("epay trade status is not successful: %s", status))
	}

	notifyAmount, err := decimal.NewFromString(input.Params["money"])
	if err != nil {
		return fail("invalid_money", order, provider, fmt.Errorf("invalid epay money: %w", err))
	}
	expectedAmount := microsToDecimal(paymentOrderPayableAmountMicros(order))
	if !notifyAmount.Equal(expectedAmount) {
		return fail("money_mismatch", order, provider, fmt.Errorf("epay money mismatch: got %s want %s", notifyAmount, expectedAmount))
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
		RecordIgnoredEvent: true,
	})
}

type recordFailedEPayNotifyInput struct {
	Params   map[string]string
	Order    *ent.PaymentOrder
	Provider *ent.PaymentProviderInstance
	Reason   string
	Err      error
}

func (s *PaymentService) recordFailedEPayNotify(ctx context.Context, input recordFailedEPayNotifyInput) error {
	eventKey := failedEPayNotifyEventKey(input.Params, input.Reason)
	if eventKey == "" {
		return nil
	}
	payload, err := json.Marshal(input.Params)
	if err != nil {
		return fmt.Errorf("failed to marshal epay notify failure payload: %w", err)
	}

	message := ""
	if input.Err != nil {
		message = input.Err.Error()
	}
	create := s.entFromContext(ctx).PaymentEvent.Create().
		SetEventKey(eventKey).
		SetProviderType(paymentevent.ProviderTypeEpay).
		SetEventType("epay_notify_failed").
		SetPayload(objects.JSONRawMessage(payload)).
		SetStatus(paymentevent.StatusFailed).
		SetError(message)
	if input.Order != nil {
		create.SetPaymentOrderID(input.Order.ID)
	}
	if input.Provider != nil {
		create.SetProviderInstanceID(input.Provider.ID)
	}

	if _, err := create.Save(ctx); ent.IsConstraintError(err) {
		_, err = s.entFromContext(ctx).PaymentEvent.Query().
			Where(paymentevent.EventKeyEQ(eventKey)).
			Only(ctx)
		return err
	} else if err != nil {
		return err
	}
	if s.notificationService != nil {
		s.notifyNonBlocking(ctx, "notify payment failure", func() error {
			return s.notificationService.NotifyPaymentFailure(ctx, input.Order, input.Reason)
		})
		s.notifyNonBlocking(ctx, "notify operator payment callback failure", func() error {
			return s.notificationService.NotifyOperatorAlert(
				ctx,
				"operator_payment_callback_failure:"+eventKey,
				"Payment callback failed",
				fmt.Sprintf("ePay callback failed with reason %s: %s", input.Reason, message),
				"",
			)
		})
	}

	return nil
}

func failedEPayNotifyEventKey(params map[string]string, reason string) string {
	if reason == "" {
		reason = "unknown"
	}
	if tradeNo := strings.TrimSpace(params["trade_no"]); tradeNo != "" {
		return fmt.Sprintf("epay_notify_failed:%s:%s", tradeNo, reason)
	}
	if orderNo := strings.TrimSpace(params["out_trade_no"]); orderNo != "" {
		return fmt.Sprintf("epay_notify_failed:order:%s:%s", orderNo, reason)
	}
	return ""
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
		SetStatus(paymentorder.StatusPending).
		SetExpiresAt(time.Now().UTC().Add(defaultPaymentOrderTTL))
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

type CancelPaymentOrderInput struct {
	OrderNo string
	Reason  string
	ActorID string
}

type MakeUpPaymentOrderInput struct {
	OrderNo string
	Reason  string
	ActorID string
	PaidAt  *time.Time
}

type AdjustUserBalanceInput struct {
	UserID         int
	Direction      ledgertransaction.Direction
	Amount         decimal.Decimal
	Currency       string
	IdempotencyKey string
	Memo           string
	ActorID        string
}

type UpdateUserBillingAccountInput struct {
	UserID      int
	Status      *billingaccount.Status
	CreditLimit *decimal.Decimal
}

func (s *PaymentService) UpdateUserBillingAccount(ctx context.Context, input UpdateUserBillingAccountInput) (*ent.BillingAccount, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}

	account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(input.UserID))
	if err != nil {
		return nil, err
	}

	update := s.entFromContext(ctx).BillingAccount.UpdateOneID(account.ID)
	if input.Status != nil {
		switch *input.Status {
		case billingaccount.StatusActive, billingaccount.StatusFrozen, billingaccount.StatusClosed:
			update.SetStatus(*input.Status)
		default:
			return nil, fmt.Errorf("unsupported billing account status %q", *input.Status)
		}
	}
	if input.CreditLimit != nil {
		if input.CreditLimit.IsNegative() {
			return nil, fmt.Errorf("credit limit cannot be negative")
		}
		creditLimitMicros, err := decimalToMicros(*input.CreditLimit)
		if err != nil {
			return nil, err
		}
		update.SetCreditLimitMicros(creditLimitMicros)
	}

	updated, err := update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user billing account: %w", err)
	}

	return updated, nil
}

func (s *PaymentService) AdjustUserBalance(ctx context.Context, input AdjustUserBalanceInput) (*ent.LedgerTransaction, error) {
	if input.UserID <= 0 {
		return nil, fmt.Errorf("user id is required")
	}
	if input.Direction != ledgertransaction.DirectionCredit && input.Direction != ledgertransaction.DirectionDebit {
		return nil, fmt.Errorf("unsupported adjustment direction %q", input.Direction)
	}
	if input.Currency == "" {
		input.Currency = defaultBillingCurrency
	}
	if input.IdempotencyKey == "" {
		randSuffix, err := randomHex(8)
		if err != nil {
			return nil, err
		}
		input.IdempotencyKey = fmt.Sprintf("admin_adjustment:user:%d:%s", input.UserID, randSuffix)
	}
	if input.Memo == "" {
		input.Memo = "admin balance adjustment"
	}

	amountMicros, err := decimalToMicros(input.Amount)
	if err != nil {
		return nil, err
	}
	if amountMicros <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	account, err := s.billingAccountService.GetOrCreateForSubject(ctx, UserBillingSubject(input.UserID))
	if err != nil {
		return nil, err
	}
	if account.Currency != input.Currency {
		return nil, fmt.Errorf("ledger currency %s does not match account currency %s", input.Currency, account.Currency)
	}

	ledgerTx, err := s.ledgerService.Post(ctx, LedgerPostInput{
		BillingAccountID: account.ID,
		Direction:        input.Direction,
		Amount:           input.Amount,
		Currency:         input.Currency,
		Type:             ledgertransaction.TypeAdminAdjustment,
		IdempotencyKey:   input.IdempotencyKey,
		ReferenceType:    "billing_account",
		ReferenceID:      fmt.Sprint(account.ID),
		Memo:             input.Memo,
		CreatedByType:    ledgertransaction.CreatedByTypeAdmin,
		CreatedByID:      input.ActorID,
	})
	if err != nil {
		return nil, err
	}
	if s.notificationService != nil && input.Direction == ledgertransaction.DirectionDebit {
		updatedAccount, loadErr := s.entFromContext(ctx).BillingAccount.Get(ctx, account.ID)
		if loadErr == nil {
			s.notifyNonBlocking(ctx, "notify low balance after admin adjustment", func() error {
				return s.notificationService.NotifyLowBalance(ctx, updatedAccount)
			})
		}
	}
	return ledgerTx, nil
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

func (s *PaymentService) CancelPaymentOrder(ctx context.Context, input CancelPaymentOrderInput) (*ent.PaymentOrder, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.OrderNo == "" {
		return nil, fmt.Errorf("order no is required")
	}
	if input.Reason == "" {
		return nil, fmt.Errorf("cancel reason is required")
	}

	var canceled *ent.PaymentOrder
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
		if order.Status == paymentorder.StatusCanceled {
			canceled = order
			return nil
		}
		if order.Status != paymentorder.StatusPending {
			return fmt.Errorf("%w: %s", ErrPaymentOrderNotPayable, order.Status)
		}

		now := time.Now().UTC()
		updated, err := client.PaymentOrder.UpdateOneID(order.ID).
			SetStatus(paymentorder.StatusCanceled).
			SetCanceledAt(now).
			SetCancelReason(input.Reason).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to cancel payment order: %w", err)
		}

		eventKey := fmt.Sprintf("payment_order_canceled:%d", order.ID)
		_, err = client.PaymentEvent.Create().
			SetEventKey(eventKey).
			SetPaymentOrderID(order.ID).
			SetNillableProviderInstanceID(order.ProviderInstanceID).
			SetProviderType(paymentevent.ProviderType(order.ProviderType)).
			SetEventType("payment_order_canceled").
			SetStatus(paymentevent.StatusProcessed).
			SetError(input.Reason).
			Save(ctx)
		if ent.IsConstraintError(err) {
			err = nil
		}
		if err != nil {
			return fmt.Errorf("failed to create payment cancellation event: %w", err)
		}

		canceled = updated
		return nil
	})
	if err != nil {
		return nil, err
	}

	return canceled, nil
}

func (s *PaymentService) MakeUpPaymentOrder(ctx context.Context, input MakeUpPaymentOrderInput) (*ent.PaymentOrder, error) {
	input.OrderNo = strings.TrimSpace(input.OrderNo)
	input.Reason = strings.TrimSpace(input.Reason)
	if input.OrderNo == "" {
		return nil, fmt.Errorf("order no is required")
	}
	if input.Reason == "" {
		return nil, fmt.Errorf("make-up reason is required")
	}
	if input.PaidAt == nil {
		now := time.Now().UTC()
		input.PaidAt = &now
	}

	return s.confirmPaidOrder(ctx, confirmPaidOrderInput{
		OrderNo:            input.OrderNo,
		EventKey:           "admin_makeup:" + input.OrderNo,
		PaidAt:             input.PaidAt,
		ProviderType:       paymentevent.ProviderTypeManual,
		EventType:          "admin_makeup",
		CreatedByType:      ledgertransaction.CreatedByTypeAdmin,
		CreatedByID:        input.ActorID,
		LedgerMemoProvider: "admin make-up",
		AllowNonPending:    true,
		MakeupReason:       input.Reason,
	})
}

func (s *PaymentService) ExpirePendingPaymentOrders(ctx context.Context, now time.Time, limit int) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if limit <= 0 {
		limit = 100
	}

	orders, err := s.entFromContext(ctx).PaymentOrder.Query().
		Where(
			paymentorder.StatusEQ(paymentorder.StatusPending),
			paymentorder.ExpiresAtNotNil(),
			paymentorder.ExpiresAtLTE(now),
		).
		Limit(limit).
		All(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired payment orders: %w", err)
	}

	var expired int
	for _, order := range orders {
		updated, err := s.entFromContext(ctx).PaymentOrder.Update().
			Where(paymentorder.IDEQ(order.ID), paymentorder.StatusEQ(paymentorder.StatusPending)).
			SetStatus(paymentorder.StatusExpired).
			Save(ctx)
		if err != nil {
			return expired, fmt.Errorf("failed to expire payment order %s: %w", order.OrderNo, err)
		}
		if updated > 0 {
			expired++
			if s.notificationService != nil {
				s.notifyNonBlocking(ctx, "notify expired payment order", func() error {
					return s.notificationService.NotifyPaymentFailure(ctx, order, "expired")
				})
			}
		}
	}

	return expired, nil
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
	AllowNonPending    bool
	RecordIgnoredEvent bool
	MakeupReason       string
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
	var terminalErr error
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
		if order.Status != paymentorder.StatusPending && !input.AllowNonPending {
			terminalErr = fmt.Errorf("%w: %s", ErrPaymentOrderNotPayable, order.Status)
			if input.RecordIgnoredEvent {
				if err := s.recordIgnoredPaymentEvent(ctx, order, input, terminalErr); err != nil {
					return err
				}
			}
			return nil
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
		if input.MakeupReason != "" {
			update.SetMakeupReason(input.MakeupReason)
		}
		if input.ExternalTradeNo != "" {
			update.SetExternalTradeNo(input.ExternalTradeNo)
		}

		paidOrder, err = update.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark payment order paid: %w", err)
		}
		if order.DiscountAmountMicros > 0 {
			_, err = client.PromoUsage.Update().
				Where(promousage.PaymentOrderIDEQ(order.ID), promousage.StatusEQ(promousage.StatusReserved)).
				SetStatus(promousage.StatusApplied).
				SetLedgerTransactionID(ledgerTx.ID).
				Save(ctx)
			if err != nil {
				return fmt.Errorf("failed to apply payment promo usage: %w", err)
			}
		}
		if s.affiliateService != nil {
			if _, err := s.affiliateService.CreateRebateForPaymentOrder(ctx, paidOrder); err != nil {
				return err
			}
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
	if terminalErr != nil {
		return nil, terminalErr
	}
	if s.notificationService != nil && paidOrder != nil {
		s.notifyNonBlocking(ctx, "notify payment success", func() error {
			return s.notificationService.NotifyPaymentSuccess(ctx, paidOrder)
		})
	}

	return paidOrder, nil
}

func (s *PaymentService) notifyNonBlocking(ctx context.Context, action string, fn func() error) {
	if err := fn(); err != nil {
		log.Warn(ctx, action+" failed", log.Cause(err))
	}
}

func (s *PaymentService) recordIgnoredPaymentEvent(ctx context.Context, order *ent.PaymentOrder, input confirmPaidOrderInput, cause error) error {
	client := s.entFromContext(ctx)
	event, err := client.PaymentEvent.Create().
		SetEventKey(input.EventKey).
		SetPaymentOrderID(order.ID).
		SetNillableProviderInstanceID(order.ProviderInstanceID).
		SetProviderType(input.ProviderType).
		SetEventType(input.EventType).
		SetPayload(input.Payload).
		SetStatus(paymentevent.StatusIgnored).
		SetError(cause.Error()).
		Save(ctx)
	if ent.IsConstraintError(err) {
		event, err = client.PaymentEvent.Query().
			Where(paymentevent.EventKeyEQ(input.EventKey)).
			Only(ctx)
	}
	if err != nil {
		return fmt.Errorf("failed to create ignored payment event: %w", err)
	}
	if event.PaymentOrderID == nil || *event.PaymentOrderID != order.ID {
		return fmt.Errorf("payment event %q does not belong to order %s", input.EventKey, input.OrderNo)
	}

	if event.Status != paymentevent.StatusIgnored {
		_, err = client.PaymentEvent.UpdateOneID(event.ID).
			SetStatus(paymentevent.StatusIgnored).
			SetError(cause.Error()).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to mark payment event ignored: %w", err)
		}
	}

	return nil
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

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random hex: %w", err)
	}

	return hex.EncodeToString(buf), nil
}
