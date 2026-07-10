package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/pkg/xerrors"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/internal/server/orchestrator"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/httpclient"
	"github.com/looplj/axonhub/llm/transformer"
	"github.com/looplj/axonhub/llm/transformer/aisdk"
)

type PlaygroundResponseError struct {
	Status int    `json:"-"`
	Reason string `json:"reason,omitempty"`
	Error  struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message"`
	} `json:"error"`
}

type PlaygroundHandlersParams struct {
	fx.In

	ChannelService              *biz.ChannelService
	UpstreamAccountService      *biz.UpstreamAccountService
	ModelService                *biz.ModelService
	DefaultSelector             *orchestrator.DefaultSelector
	RequestService              *biz.RequestService
	SystemService               *biz.SystemService
	UsageLogService             *biz.UsageLogService
	PromptService               *biz.PromptService
	PromptProtectionRuleService *biz.PromptProtectionRuleService
	QuotaService                *biz.QuotaService
	HttpClient                  *httpclient.HttpClient
	LiveStreamRegistry          *biz.LiveStreamRegistry
	ChannelLimiterManager       *orchestrator.ChannelLimiterManager
	ProviderQuotaStatusProvider orchestrator.ProviderQuotaStatusProvider
	AdmissionService            *biz.AdmissionService
	UsageBillingProcessor       *biz.UsageBillingProcessor
	BillingHoldService          *biz.BillingHoldService
}

type PlaygroundHandlers struct {
	ChannelService             *biz.ChannelService
	ChatCompletionOrchestrator *orchestrator.ChatCompletionOrchestrator
}

const consumerPlaygroundContextKey = "consumer_playground"

func NewPlaygroundHandlers(params PlaygroundHandlersParams) *PlaygroundHandlers {
	return &PlaygroundHandlers{
		ChannelService: params.ChannelService,
		ChatCompletionOrchestrator: orchestrator.NewChatCompletionOrchestrator(
			params.ChannelService,
			params.DefaultSelector,
			params.RequestService,
			params.HttpClient,
			aisdk.NewDataStreamTransformer(),
			params.SystemService,
			params.UsageLogService,
			params.PromptService,
			params.QuotaService,
			params.PromptProtectionRuleService,
			params.LiveStreamRegistry,
			params.ChannelLimiterManager,
			params.ProviderQuotaStatusProvider,
			orchestrator.WithCommercialBilling(params.AdmissionService, params.UsageBillingProcessor, params.BillingHoldService),
			orchestrator.WithUpstreamAccounts(params.UpstreamAccountService),
		),
	}
}

// tryExtractUpstreamErrorMessage attempts to extract a meaningful error message
// from a typical upstream error JSON payload. Supported shapes include:
// {"error": {"message": "..."}}, {"message": "..."}
// If nothing is extracted, returns an empty string.
func tryExtractUpstreamErrorMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}

	// 1) OpenAI / OpenRouter-like: {"error": {"message": "..."}}
	var wrapped struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &wrapped); err == nil {
		if wrapped.Error.Message != "" {
			return wrapped.Error.Message
		}
	}

	// 2) Generic: {"message": "..."}
	var generic struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &generic); err == nil {
		if generic.Message != "" {
			return generic.Message
		}
	}

	// 3) Some providers may return: {"error": "..."}
	var alt struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(body, &alt); err == nil {
		if alt.Error != "" {
			return alt.Error
		}
	}

	return ""
}

// HandleError handles raw errors and returns a PlaygroundResponseError.
func (handlers *PlaygroundHandlers) HandleError(rawErr error) *PlaygroundResponseError {
	var quotaErr *orchestrator.QuotaExhaustedError
	if errors.As(rawErr, &quotaErr) {
		return &PlaygroundResponseError{
			Status: http.StatusServiceUnavailable,
			Reason: "upstream_unavailable",
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    http.StatusServiceUnavailable,
				Message: quotaErr.Error(),
			},
		}
	}

	if httpErr, ok := xerrors.As[*httpclient.Error](rawErr); ok {
		// Prefer upstream error message when available
		msg := tryExtractUpstreamErrorMessage(httpErr.Body)
		if msg == "" {
			msg = http.StatusText(httpErr.StatusCode)
		}

		return &PlaygroundResponseError{
			Status: httpErr.StatusCode,
			Reason: playgroundHTTPErrorReason(httpErr.StatusCode),
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    httpErr.StatusCode,
				Message: msg,
			},
		}
	}

	// Handle validation errors
	if errors.Is(rawErr, transformer.ErrInvalidRequest) {
		return &PlaygroundResponseError{
			Status: http.StatusBadRequest,
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    http.StatusBadRequest,
				Message: http.StatusText(http.StatusBadRequest),
			},
		}
	}

	if llmErr, ok := xerrors.As[*llm.ResponseError](rawErr); ok && llmErr != nil {
		reason := playgroundLLMErrorReason(llmErr)
		if llmErr.Detail.Message == "" {
			return &PlaygroundResponseError{
				Status: llmErr.StatusCode,
				Reason: reason,
				Error: struct {
					Code    int    `json:"code,omitempty"`
					Message string `json:"message"`
				}{
					Code:    llmErr.StatusCode,
					Message: http.StatusText(llmErr.StatusCode),
				},
			}
		}

		// Try parse provider error code if present and numeric; otherwise use HTTP status.
		parsedCode, _ := strconv.Atoi(llmErr.Detail.Code)
		if parsedCode == 0 {
			parsedCode = llmErr.StatusCode
		}

		return &PlaygroundResponseError{
			Status: llmErr.StatusCode,
			Reason: reason,
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    parsedCode,
				Message: llmErr.Detail.Message,
			},
		}
	}

	return &PlaygroundResponseError{
		Status: http.StatusInternalServerError,
		Reason: "upstream_unavailable",
		Error: struct {
			Code    int    `json:"code,omitempty"`
			Message string `json:"message"`
		}{
			Code:    http.StatusInternalServerError,
			Message: http.StatusText(http.StatusInternalServerError),
		},
	}
}

func playgroundHTTPErrorReason(status int) string {
	if status == http.StatusTooManyRequests {
		return "rate_limited"
	}
	if status >= http.StatusInternalServerError {
		return "upstream_unavailable"
	}
	return ""
}

func playgroundLLMErrorReason(err *llm.ResponseError) string {
	switch err.Detail.Code {
	case string(biz.AdmissionCodeInsufficientBalance), string(biz.AdmissionCodeSubscriptionExhausted):
		return "balance_insufficient"
	case string(biz.AdmissionCodeAPIKeyBudgetExceeded):
		return "key_disabled"
	}
	return playgroundHTTPErrorReason(err.StatusCode)
}

func (handlers *PlaygroundHandlers) ChatCompletion(c *gin.Context) {
	ctx := c.Request.Context()

	genericReq, err := httpclient.ReadHTTPRequest(c.Request)
	if err != nil {
		log.Error(ctx, "Error reading HTTP request", log.Cause(err))
		c.JSON(http.StatusBadRequest, PlaygroundResponseError{
			Error: struct {
				Code    int    `json:"code,omitempty"`
				Message string `json:"message"`
			}{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		})

		return
	}

	var channelIDStr, projectIDStr string
	if !c.GetBool(consumerPlaygroundContextKey) {
		channelIDStr = c.Query("channel_id")
		if channelIDStr == "" {
			channelIDStr = c.GetHeader("X-Channel-ID")
		}

		// Extract project ID from header
		projectIDStr = c.Query("project_id")
		if projectIDStr == "" {
			projectIDStr = c.GetHeader("X-Project-ID")
		}
	}

	// Parse and set project ID in context if provided
	if projectIDStr != "" {
		projectID, err := objects.ParseGUID(projectIDStr)
		if err != nil {
			log.Error(ctx, "Error parsing project ID", log.Cause(err))
			c.JSON(http.StatusBadRequest, PlaygroundResponseError{
				Error: struct {
					Code    int    `json:"code,omitempty"`
					Message string `json:"message"`
				}{
					Code:    http.StatusBadRequest,
					Message: "Invalid project ID: " + err.Error(),
				},
			})

			return
		}

		ctx = contexts.WithProjectID(ctx, projectID.ID)
		// Update the request context
		c.Request = c.Request.WithContext(ctx)
	}

	log.Debug(ctx, "Received request", log.Any("request", genericReq), log.String("channel_id", channelIDStr), log.String("project_id", projectIDStr))

	processor := handlers.ChatCompletionOrchestrator

	if channelIDStr != "" {
		channelID, err := objects.ParseGUID(channelIDStr)
		if err != nil {
			log.Error(ctx, "Error parsing channel ID", log.Cause(err))
			c.JSON(http.StatusBadRequest, PlaygroundResponseError{
				Error: struct {
					Code    int    `json:"code,omitempty"`
					Message string `json:"message"`
				}{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			})

			return
		}

		processor = processor.WithChannelSelector(orchestrator.NewSpecifiedChannelSelector(handlers.ChannelService, channelID))
	}

	result, err := processor.Process(ctx, genericReq)
	if err != nil {
		log.Error(ctx, "Error processing chat completion", log.Cause(err))
		errResponse := handlers.HandleError(err)
		c.JSON(errResponse.Status, errResponse)

		return
	}

	if result.ChatCompletion != nil {
		resp := result.ChatCompletion

		contentType := "application/json"
		if ct := resp.Headers.Get("Content-Type"); ct != "" {
			contentType = ct
		}

		c.Data(resp.StatusCode, contentType, resp.Body)

		return
	}

	if result.ChatCompletionStream != nil {
		defer func() {
			err := result.ChatCompletionStream.Close()
			if err != nil {
				log.Error(ctx, "Error closing stream", log.Cause(err))
			}
		}()

		// Set AI SDK data stream headers

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("X-Vercel-AI-Data-Stream", "v1")
		c.Status(http.StatusOK)

		WriteSSEStream(c, result.ChatCompletionStream)
	}
}
