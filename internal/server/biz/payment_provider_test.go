package biz

import (
	"context"
	"crypto/md5" //nolint:gosec // ePay uses MD5 signatures.
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/paymentorder"
	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/objects"
)

func TestPaymentProviderRegistryResolvesBuiltInAdapters(t *testing.T) {
	registry := NewPaymentProviderRegistry()

	manual, err := registry.Adapter(paymentproviderinstance.ProviderTypeManual)
	require.NoError(t, err)
	require.Equal(t, paymentproviderinstance.ProviderTypeManual, manual.ProviderType())

	epay, err := registry.Adapter(paymentproviderinstance.ProviderTypeEpay)
	require.NoError(t, err)
	require.Equal(t, paymentproviderinstance.ProviderTypeEpay, epay.ProviderType())
}

func TestEPaySignatureSortsParamsAndSkipsSignFields(t *testing.T) {
	params := map[string]string{
		"money":        "12.34",
		"pid":          "1001",
		"out_trade_no": "pay_1",
		"sign":         "old",
		"sign_type":    "MD5",
		"empty":        "",
	}

	base := "money=12.34&out_trade_no=pay_1&pid=1001"
	sum := md5.Sum([]byte(base + "secret"))
	expected := strings.ToLower(hex.EncodeToString(sum[:]))

	sign := SignEPayParams(params, "secret")
	require.Equal(t, expected, sign)

	params["sign"] = sign
	require.True(t, VerifyEPaySignature(params, "secret"))

	params["money"] = "99.00"
	require.False(t, VerifyEPaySignature(params, "secret"))
}

func TestEPayAdapterCreateCheckout(t *testing.T) {
	cfg := EPayConfig{
		GatewayURL: "https://pay.example.com/submit.php",
		PID:        "1001",
		Key:        "secret",
		NotifyURL:  "https://axon.example.com/payment/notify/epay",
		ReturnURL:  "https://axon.example.com/billing",
		Type:       "wxpay",
		SiteName:   "AxonHub",
	}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)

	adapter := EPayPaymentProviderAdapter{}
	checkout, err := adapter.CreateCheckout(context.Background(), PaymentProviderCheckoutInput{
		ProviderInstance: &ent.PaymentProviderInstance{
			ProviderType: paymentproviderinstance.ProviderTypeEpay,
			Config:       objects.JSONRawMessage(raw),
		},
		Order: &ent.PaymentOrder{
			OrderNo:      "pay_123",
			ProviderType: paymentorder.ProviderTypeEpay,
			AmountMicros: 12_340_000,
			Currency:     "CNY",
		},
		Subject: "Recharge 12.34",
	})
	require.NoError(t, err)
	require.Equal(t, paymentproviderinstance.ProviderTypeEpay, checkout.ProviderType)
	require.Equal(t, "redirect", checkout.Method)
	require.Equal(t, "12.34", checkout.Params["money"])
	require.Equal(t, "pay_123", checkout.Params["out_trade_no"])
	require.Equal(t, "MD5", checkout.Params["sign_type"])
	require.True(t, VerifyEPaySignature(checkout.Params, cfg.Key))
	require.Contains(t, checkout.URL, cfg.GatewayURL+"?")
	require.Contains(t, checkout.URL, "out_trade_no=pay_123")
}
