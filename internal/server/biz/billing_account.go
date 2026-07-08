package biz

import (
	"context"
	"fmt"

	"go.uber.org/fx"

	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/billingaccount"
	"github.com/looplj/axonhub/internal/ent/billingaccountbinding"
)

const (
	BillingSubjectTypeUser    = "user"
	BillingSubjectTypeProject = "project"
	defaultBillingCurrency    = "CNY"
)

type BillingSubject struct {
	Type string
	ID   int
}

func ProjectBillingSubject(projectID int) BillingSubject {
	return BillingSubject{Type: BillingSubjectTypeProject, ID: projectID}
}

func UserBillingSubject(userID int) BillingSubject {
	return BillingSubject{Type: BillingSubjectTypeUser, ID: userID}
}

func (s BillingSubject) validate() error {
	if s.Type != BillingSubjectTypeUser && s.Type != BillingSubjectTypeProject {
		return fmt.Errorf("unsupported billing subject type %q", s.Type)
	}
	if s.ID <= 0 {
		return fmt.Errorf("billing subject id must be positive")
	}

	return nil
}

type BillingAccountServiceParams struct {
	fx.In

	Ent *ent.Client
}

type BillingAccountService struct {
	*AbstractService
}

func NewBillingAccountService(params BillingAccountServiceParams) *BillingAccountService {
	return &BillingAccountService{
		AbstractService: &AbstractService{db: params.Ent},
	}
}

func (s *BillingAccountService) GetBySubject(ctx context.Context, subject BillingSubject) (*ent.BillingAccount, error) {
	if err := subject.validate(); err != nil {
		return nil, err
	}

	client := s.entFromContext(ctx)
	account, err := client.BillingAccount.Query().
		Where(
			billingaccount.OwnerTypeEQ(billingaccount.OwnerType(subject.Type)),
			billingaccount.OwnerIDEQ(subject.ID),
		).
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, fmt.Errorf("%w: %s:%d", ErrBillingAccountNotFound, subject.Type, subject.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get billing account: %w", err)
	}

	return account, nil
}

func (s *BillingAccountService) GetOrCreateForSubject(ctx context.Context, subject BillingSubject) (*ent.BillingAccount, error) {
	if err := subject.validate(); err != nil {
		return nil, err
	}

	var account *ent.BillingAccount
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		existing, err := client.BillingAccount.Query().
			Where(
				billingaccount.OwnerTypeEQ(billingaccount.OwnerType(subject.Type)),
				billingaccount.OwnerIDEQ(subject.ID),
			).
			Only(ctx)
		if err == nil {
			account = existing
			return nil
		}
		if !ent.IsNotFound(err) {
			return fmt.Errorf("failed to query billing account: %w", err)
		}

		created, err := client.BillingAccount.Create().
			SetOwnerType(billingaccount.OwnerType(subject.Type)).
			SetOwnerID(subject.ID).
			SetCurrency(defaultBillingCurrency).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create billing account: %w", err)
		}

		_, err = client.BillingAccountBinding.Create().
			SetBillingAccountID(created.ID).
			SetOwnerType(billingaccountbinding.OwnerType(subject.Type)).
			SetOwnerID(subject.ID).
			SetRelation("primary").
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create billing account binding: %w", err)
		}

		account = created
		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}
