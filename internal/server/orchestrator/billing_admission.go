package orchestrator

import (
	"context"
	"net/http"

	"github.com/looplj/axonhub/internal/contexts"
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
		})
		if decision.Allowed {
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
				Code:      "billing_admission_denied",
				Message:   decision.Reason,
				Type:      "billing_error",
				RequestID: requestID,
			},
		}
	})
}
