package gql

import (
	"encoding/json"
	"fmt"

	"github.com/looplj/axonhub/internal/ent/paymentproviderinstance"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func paymentCheckoutFromBiz(checkout *biz.PaymentProviderCheckout) (*PaymentCheckout, error) {
	if checkout == nil {
		return nil, fmt.Errorf("payment checkout is nil")
	}

	params, err := json.Marshal(checkout.Params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal checkout params: %w", err)
	}

	var urlValue *string
	if checkout.URL != "" {
		urlValue = &checkout.URL
	}

	return &PaymentCheckout{
		ProviderType: string(checkout.ProviderType),
		OrderNo:      checkout.OrderNo,
		Method:       checkout.Method,
		URL:          urlValue,
		Params:       objects.JSONRawMessage(params),
		Amount:       checkout.Amount,
		Currency:     checkout.Currency,
	}, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}

func paymentProviderStatusValue(value *paymentproviderinstance.Status) paymentproviderinstance.Status {
	if value == nil {
		return ""
	}

	return *value
}

func paymentProviderInstanceIDValue(value *objects.GUID) *int {
	if value == nil {
		return nil
	}

	return &value.ID
}
