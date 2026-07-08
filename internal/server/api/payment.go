package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/server/biz"
)

type PaymentHandlersParams struct {
	fx.In

	PaymentService *biz.PaymentService
}

type PaymentHandlers struct {
	PaymentService *biz.PaymentService
}

func NewPaymentHandlers(params PaymentHandlersParams) *PaymentHandlers {
	return &PaymentHandlers{PaymentService: params.PaymentService}
}

func (h *PaymentHandlers) NotifyEPay(c *gin.Context) {
	params, err := collectEPayParams(c)
	if err != nil {
		c.String(http.StatusBadRequest, "fail")
		return
	}

	if _, err := h.PaymentService.HandleEPayNotify(c.Request.Context(), biz.HandleEPayNotifyInput{Params: params}); err != nil {
		_ = c.Error(err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	c.String(http.StatusOK, "success")
}

func (h *PaymentHandlers) SimulateEPaySubmit(c *gin.Context) {
	params := biz.EPayParamsFromValues(c.Request.URL.Query())
	if params["out_trade_no"] == "" || params["pid"] == "" || params["money"] == "" {
		JSONError(c, http.StatusBadRequest, errors.New("missing epay checkout params"))
		return
	}

	notifyURL := params["notify_url"]
	if notifyURL == "" {
		JSONError(c, http.StatusBadRequest, errors.New("missing notify_url"))
		return
	}

	notifyParams := biz.NewSimulatedEPayNotifyFromCheckout(params, "axonhub-simulated-epay-secret")
	if _, err := h.PaymentService.HandleEPayNotify(c.Request.Context(), biz.HandleEPayNotifyInput{Params: notifyParams}); err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	returnURL := params["return_url"]
	if returnURL == "" {
		c.JSON(http.StatusOK, gin.H{
			"status":         "success",
			"out_trade_no":   params["out_trade_no"],
			"simulated_post": notifyURL,
			"notify_params":  notifyParams,
		})
		return
	}

	redirectURL, err := appendEPayReturnParams(returnURL, notifyParams)
	if err != nil {
		JSONError(c, http.StatusBadRequest, err)
		return
	}

	c.Redirect(http.StatusFound, redirectURL)
}

func collectEPayParams(c *gin.Context) (map[string]string, error) {
	params := biz.EPayParamsFromValues(c.Request.URL.Query())
	if c.Request.Method == http.MethodPost {
		if err := c.Request.ParseForm(); err != nil {
			return nil, fmt.Errorf("failed to parse form: %w", err)
		}
		for key := range c.Request.PostForm {
			params[key] = c.Request.PostForm.Get(key)
		}
	}

	return params, nil
}

func appendEPayReturnParams(rawURL string, params map[string]string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid return_url: %w", err)
	}

	values := u.Query()
	for key, value := range params {
		if value != "" {
			values.Set(key, value)
		}
	}
	u.RawQuery = values.Encode()

	return u.String(), nil
}
