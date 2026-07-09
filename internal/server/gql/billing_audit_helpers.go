package gql

import (
	"context"

	"github.com/looplj/axonhub/internal/server/biz"
)

func (r *mutationResolver) auditBillingAdminAction(ctx context.Context, input biz.BillingAuditInput) {
	if r.billingAuditService == nil {
		return
	}
	_, _ = r.billingAuditService.RecordAdmin(ctx, input)
}
