package orchestrator

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/log"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
	"github.com/looplj/axonhub/llm/pipeline"
)

func enforceBillingAdmission(inbound *PersistentInboundTransformer) pipeline.Middleware {
	return pipeline.OnLlmRequest("billing-admission", func(ctx context.Context, llmRequest *llm.Request) (*llm.Request, error) {
		state := inbound.state
		if state.AdmissionService == nil {
			return llmRequest, nil
		}

		projectID, ok := contexts.GetProjectID(ctx)
		if !ok && state.APIKey != nil {
			projectID = state.APIKey.ProjectID
			ok = projectID > 0
		}
		if !ok {
			return llmRequest, nil
		}

		subject, ok := state.AdmissionService.BillingSubjectForAPIKey(state.APIKey, projectID)
		if !ok {
			return llmRequest, nil
		}

		decision, err := state.AdmissionService.Check(ctx, biz.AdmissionCheckInput{
			Subject: subject,
			ModelID: llmRequest.Model,
			APIKey:  state.APIKey,
		})
		if decision.Allowed {
			if decision.Code == biz.AdmissionCodeAllowed && state.BillingHoldService != nil {
				if state.Request == nil {
					request, err := state.RequestService.CreateRequest(
						ctx,
						llmRequest,
						state.RawRequest,
						llmRequest.APIFormat,
					)
					if err != nil {
						return nil, err
					}
					state.Request = request
				}

				hold, err := state.BillingHoldService.CreateRequestHold(ctx, biz.CreateRequestBillingHoldInput{
					Subject:   subject,
					RequestID: state.Request.ID,
					ProjectID: projectID,
					APIKeyID:  apiKeyIDPtr(state.APIKey),
					ModelID:   llmRequest.Model,
					MaxTokens: requestMaxTokens(llmRequest),
				})
				if err != nil {
					if decision.Mode == biz.AdmissionModeWarn {
						log.Warn(ctx, "billing hold warning",
							log.Int("project_id", projectID),
							log.String("model_id", llmRequest.Model),
							log.Cause(err),
						)
						return llmRequest, nil
					}

					requestID, _ := contexts.GetRequestID(ctx)
					log.Info(ctx, "billing hold blocked request",
						log.Int("project_id", projectID),
						log.String("model_id", llmRequest.Model),
						log.Cause(err),
					)

					return nil, &llm.ResponseError{
						StatusCode: http.StatusPaymentRequired,
						Detail: llm.ErrorDetail{
							Code:      string(biz.AdmissionCodeInsufficientBalance),
							Message:   err.Error(),
							Type:      "billing_error",
							RequestID: requestID,
						},
					}
				}
				state.BillingHold = hold
			}

			if decision.Reason != "" && decision.Reason != "allowed" && decision.Mode == biz.AdmissionModeWarn {
				log.Warn(ctx, "billing admission warning",
					log.Int("project_id", projectID),
					log.String("model_id", llmRequest.Model),
					log.String("reason", decision.Reason),
				)
			}

			return llmRequest, nil
		}
		if err == nil {
			err = biz.ErrInsufficientBalance
		}

		requestID, _ := contexts.GetRequestID(ctx)
		log.Info(ctx, "billing admission blocked request",
			log.Int("project_id", projectID),
			log.String("model_id", llmRequest.Model),
			log.String("reason", decision.Reason),
			log.Cause(err),
		)

		return nil, &llm.ResponseError{
			StatusCode: http.StatusPaymentRequired,
			Detail: llm.ErrorDetail{
				Code:      string(decision.Code),
				Message:   decision.Reason,
				Type:      "billing_error",
				RequestID: requestID,
			},
		}
	})
}

func requestMaxTokens(request *llm.Request) int64 {
	if request == nil {
		return 0
	}
	if request.MaxCompletionTokens != nil && *request.MaxCompletionTokens > 0 {
		return *request.MaxCompletionTokens
	}
	if request.MaxTokens != nil && *request.MaxTokens > 0 {
		return *request.MaxTokens
	}

	return 0
}

func apiKeyIDPtr(apiKey *ent.APIKey) *int {
	if apiKey == nil || apiKey.ID <= 0 {
		return nil
	}

	return &apiKey.ID
}
