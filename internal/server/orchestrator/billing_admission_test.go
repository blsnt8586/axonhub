package orchestrator

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/server/biz"
	"github.com/looplj/axonhub/llm"
)

func TestBillingAdmission_NoServiceAllowsRequest(t *testing.T) {
	t.Parallel()

	middleware := enforceBillingAdmission(&PersistentInboundTransformer{
		state: &PersistenceState{},
	})
	req := &llm.Request{Model: "gpt-test"}

	out, err := middleware.OnInboundLlmRequest(context.Background(), req)
	require.NoError(t, err)
	require.Same(t, req, out)
}

func TestBillingAdmission_EnforceRejectsInsufficientBalance(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_admission_reject?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	ctx = contexts.WithProjectID(ctx, 1)

	accountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	_, err := accountSvc.GetOrCreateForSubject(ctx, biz.UserBillingSubject(99))
	require.NoError(t, err)
	apiKey := &ent.APIKey{ID: 7, UserID: 99, ProjectID: 1}

	admissionSvc := biz.NewAdmissionService(biz.AdmissionServiceParams{
		Config: biz.BillingConfig{
			Mode: biz.AdmissionModeEnforce,
		},
		BillingAccountService: accountSvc,
	})

	middleware := enforceBillingAdmission(&PersistentInboundTransformer{
		state: &PersistenceState{
			AdmissionService: admissionSvc,
			APIKey:           apiKey,
		},
	})

	out, err := middleware.OnInboundLlmRequest(ctx, &llm.Request{Model: "gpt-test"})
	require.Nil(t, out)

	var respErr *llm.ResponseError
	require.ErrorAs(t, err, &respErr)
	require.Equal(t, http.StatusPaymentRequired, respErr.StatusCode)
	require.Equal(t, string(biz.AdmissionCodeInsufficientBalance), respErr.Detail.Code)
	require.Equal(t, "billing_error", respErr.Detail.Type)
	require.Equal(t, "insufficient billing balance", respErr.Detail.Message)
}

func TestBillingAdmission_UserBillingSkipsRequestWithoutUserSubject(t *testing.T) {
	t.Parallel()

	client := enttest.NewEntClient(t, "sqlite3", "file:billing_admission_no_user?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	ctx = contexts.WithProjectID(ctx, 1)

	accountSvc := biz.NewBillingAccountService(biz.BillingAccountServiceParams{Ent: client})
	_, err := accountSvc.GetOrCreateForSubject(ctx, biz.ProjectBillingSubject(1))
	require.NoError(t, err)

	admissionSvc := biz.NewAdmissionService(biz.AdmissionServiceParams{
		Config: biz.BillingConfig{
			Mode: biz.AdmissionModeEnforce,
		},
		BillingAccountService: accountSvc,
	})

	middleware := enforceBillingAdmission(&PersistentInboundTransformer{
		state: &PersistenceState{
			AdmissionService: admissionSvc,
			APIKey:           &ent.APIKey{ID: 8, ProjectID: 1},
		},
	})
	req := &llm.Request{Model: "gpt-test"}

	out, err := middleware.OnInboundLlmRequest(ctx, req)
	require.NoError(t, err)
	require.Same(t, req, out)
}
