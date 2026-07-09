package biz

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingauditlog"
	"github.com/looplj/axonhub/internal/objects"
)

type BillingAuditServiceParams struct {
	fx.In

	Ent *ent.Client
}

type BillingAuditService struct {
	*AbstractService
}

func NewBillingAuditService(params BillingAuditServiceParams) *BillingAuditService {
	return &BillingAuditService{AbstractService: &AbstractService{db: params.Ent}}
}

type BillingAuditInput struct {
	Action       string
	TargetType   string
	TargetID     string
	TargetUserID *int
	Reason       string
	Metadata     objects.JSONRawMessage
}

func (s *BillingAuditService) RecordAdmin(ctx context.Context, input BillingAuditInput) (*ent.BillingAuditLog, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("admin user is required for billing audit")
	}
	input.Action = strings.TrimSpace(input.Action)
	if input.Action == "" {
		return nil, fmt.Errorf("billing audit action is required")
	}

	create := s.entFromContext(ctx).BillingAuditLog.Create().
		SetAction(input.Action).
		SetActorType(billingauditlog.ActorTypeAdmin).
		SetActorUserID(user.ID).
		SetTargetType(strings.TrimSpace(input.TargetType)).
		SetTargetID(strings.TrimSpace(input.TargetID)).
		SetReason(strings.TrimSpace(input.Reason))
	if input.TargetUserID != nil && *input.TargetUserID > 0 {
		create.SetTargetUserID(*input.TargetUserID)
	}
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}
	return create.Save(ctx)
}

func (s *BillingAuditService) RecordSystem(ctx context.Context, input BillingAuditInput) (*ent.BillingAuditLog, error) {
	input.Action = strings.TrimSpace(input.Action)
	if input.Action == "" {
		return nil, fmt.Errorf("billing audit action is required")
	}

	create := s.entFromContext(ctx).BillingAuditLog.Create().
		SetAction(input.Action).
		SetActorType(billingauditlog.ActorTypeSystem).
		SetTargetType(strings.TrimSpace(input.TargetType)).
		SetTargetID(strings.TrimSpace(input.TargetID)).
		SetReason(strings.TrimSpace(input.Reason))
	if input.TargetUserID != nil && *input.TargetUserID > 0 {
		create.SetTargetUserID(*input.TargetUserID)
	}
	if len(input.Metadata) > 0 {
		create.SetMetadata(input.Metadata)
	}
	return create.Save(ctx)
}

func AuditTargetID(id int) string {
	if id <= 0 {
		return ""
	}
	return strconv.Itoa(id)
}
