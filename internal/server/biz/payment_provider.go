package biz

import (
	"context"
	"crypto/md5" //nolint:gosec // ePay uses MD5 signatures.
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/objects"
)

type PaymentProviderAdapter interface {
	ProviderType() paymentproviderinstance.ProviderType
	CreateCheckout(ctx context.Context, input PaymentProviderCheckoutInput) (*PaymentProviderCheckout, error)
}

type PaymentProviderCheckoutInput struct {
	ProviderInstance *ent.PaymentProviderInstance
	Order            *ent.PaymentOrder
	Subject          string
}

type PaymentProviderCheckout struct {
	ProviderType paymentproviderinstance.ProviderType
	OrderNo      string
	Method       string
	URL          string
	Params       map[string]string
	Amount       decimal.Decimal
	Currency     string
}

type PaymentProviderRegistry struct {
	adapters map[paymentproviderinstance.ProviderType]PaymentProviderAdapter
}

func NewPaymentProviderRegistry() *PaymentProviderRegistry {
	registry := &PaymentProviderRegistry{
		adapters: make(map[paymentproviderinstance.ProviderType]PaymentProviderAdapter),
	}

	registry.MustRegister(ManualPaymentProviderAdapter{})
	registry.MustRegister(EPayPaymentProviderAdapter{})

	return registry
}

func (r *PaymentProviderRegistry) MustRegister(adapter PaymentProviderAdapter) {
	if err := r.Register(adapter); err != nil {
		panic(err)
	}
}

func (r *PaymentProviderRegistry) Register(adapter PaymentProviderAdapter) error {
	if adapter == nil {
		return fmt.Errorf("payment provider adapter is nil")
	}
	providerType := adapter.ProviderType()
	if providerType == "" {
		return fmt.Errorf("payment provider adapter type is empty")
	}
	if _, exists := r.adapters[providerType]; exists {
		return fmt.Errorf("payment provider adapter %q already registered", providerType)
	}

	r.adapters[providerType] = adapter
	return nil
}

func (r *PaymentProviderRegistry) Adapter(providerType paymentproviderinstance.ProviderType) (PaymentProviderAdapter, error) {
	adapter, ok := r.adapters[providerType]
	if !ok {
		return nil, fmt.Errorf("payment provider adapter %q is not registered", providerType)
	}

	return adapter, nil
}

type ManualPaymentProviderAdapter struct{}

func (ManualPaymentProviderAdapter) ProviderType() paymentproviderinstance.ProviderType {
	return paymentproviderinstance.ProviderTypeManual
}

func (ManualPaymentProviderAdapter) CreateCheckout(ctx context.Context, input PaymentProviderCheckoutInput) (*PaymentProviderCheckout, error) {
	if input.Order == nil {
		return nil, fmt.Errorf("payment order is required")
	}

	return &PaymentProviderCheckout{
		ProviderType: paymentproviderinstance.ProviderTypeManual,
		OrderNo:      input.Order.OrderNo,
		Method:       "manual",
		Amount:       microsToDecimal(paymentOrderPayableAmountMicros(input.Order)),
		Currency:     input.Order.Currency,
	}, nil
}

type EPayConfig struct {
	GatewayURL string `json:"gateway_url"`
	PID        string `json:"pid"`
	Key        string `json:"key"`
	NotifyURL  string `json:"notify_url"`
	ReturnURL  string `json:"return_url"`
	Type       string `json:"type"`
	SiteName   string `json:"site_name"`
}

type EPayNotify struct {
	PID         string `json:"pid"`
	TradeNo     string `json:"trade_no"`
	OutTradeNo  string `json:"out_trade_no"`
	Type        string `json:"type"`
	Name        string `json:"name"`
	Money       string `json:"money"`
	TradeStatus string `json:"trade_status"`
	Sign        string `json:"sign"`
	SignType    string `json:"sign_type"`
}

type EPayPaymentProviderAdapter struct{}

func (EPayPaymentProviderAdapter) ProviderType() paymentproviderinstance.ProviderType {
	return paymentproviderinstance.ProviderTypeEpay
}

func (EPayPaymentProviderAdapter) CreateCheckout(ctx context.Context, input PaymentProviderCheckoutInput) (*PaymentProviderCheckout, error) {
	if input.ProviderInstance == nil {
		return nil, fmt.Errorf("payment provider instance is required")
	}
	if input.Order == nil {
		return nil, fmt.Errorf("payment order is required")
	}

	cfg, err := parseEPayConfig(input.ProviderInstance.Config)
	if err != nil {
		return nil, err
	}

	subject := input.Subject
	if subject == "" {
		subject = "AxonHub recharge"
	}

	params := map[string]string{
		"pid":          cfg.PID,
		"type":         valueOrDefaultString(cfg.Type, "alipay"),
		"out_trade_no": input.Order.OrderNo,
		"notify_url":   cfg.NotifyURL,
		"return_url":   cfg.ReturnURL,
		"name":         subject,
		"money":        microsToDecimal(paymentOrderPayableAmountMicros(input.Order)).StringFixedBank(2),
		"sitename":     cfg.SiteName,
	}
	params["sign"] = SignEPayParams(params, cfg.Key)
	params["sign_type"] = "MD5"

	return &PaymentProviderCheckout{
		ProviderType: paymentproviderinstance.ProviderTypeEpay,
		OrderNo:      input.Order.OrderNo,
		Method:       "redirect",
		URL:          buildEPayURL(cfg.GatewayURL, params),
		Params:       params,
		Amount:       microsToDecimal(paymentOrderPayableAmountMicros(input.Order)),
		Currency:     input.Order.Currency,
	}, nil
}

func paymentOrderPayableAmountMicros(order *ent.PaymentOrder) int64 {
	if order == nil {
		return 0
	}
	if order.PayableAmountMicros > 0 || order.DiscountAmountMicros > 0 {
		return order.PayableAmountMicros
	}
	return order.AmountMicros
}

func parseEPayConfig(raw objects.JSONRawMessage) (*EPayConfig, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("epay config is required")
	}

	var cfg EPayConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse epay config: %w", err)
	}
	if cfg.GatewayURL == "" {
		return nil, fmt.Errorf("epay gateway_url is required")
	}
	if cfg.PID == "" {
		return nil, fmt.Errorf("epay pid is required")
	}
	if cfg.Key == "" {
		return nil, fmt.Errorf("epay key is required")
	}
	if cfg.NotifyURL == "" {
		return nil, fmt.Errorf("epay notify_url is required")
	}
	if cfg.ReturnURL == "" {
		return nil, fmt.Errorf("epay return_url is required")
	}

	return &cfg, nil
}

func SignEPayParams(params map[string]string, key string) string {
	base := buildEPaySignBase(params)
	sum := md5.Sum([]byte(base + key)) //nolint:gosec // ePay protocol requires MD5.
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func VerifyEPaySignature(params map[string]string, key string) bool {
	sign := strings.ToLower(params["sign"])
	if sign == "" {
		return false
	}

	return sign == SignEPayParams(params, key)
}

func NewSimulatedEPayNotifyFromCheckout(params map[string]string, key string) map[string]string {
	notify := map[string]string{
		"pid":          params["pid"],
		"trade_no":     "sim_" + params["out_trade_no"],
		"out_trade_no": params["out_trade_no"],
		"type":         params["type"],
		"name":         params["name"],
		"money":        params["money"],
		"trade_status": "TRADE_SUCCESS",
	}
	notify["sign"] = SignEPayParams(notify, key)
	notify["sign_type"] = "MD5"

	return notify
}

func EPayParamsFromValues(values url.Values) map[string]string {
	params := make(map[string]string, len(values))
	for key := range values {
		params[key] = values.Get(key)
	}

	return params
}

func buildEPaySignBase(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || k == "sign_type" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+params[k])
	}

	return strings.Join(parts, "&")
}

func buildEPayURL(gatewayURL string, params map[string]string) string {
	values := url.Values{}
	for k, v := range params {
		if v == "" {
			continue
		}
		values.Set(k, v)
	}

	sep := "?"
	if strings.Contains(gatewayURL, "?") {
		sep = "&"
	}

	return gatewayURL + sep + values.Encode()
}

func valueOrDefaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}

	return value
}
