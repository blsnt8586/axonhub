package gql

import (
	"context"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent/userproject"
)

func (r *mutationResolver) requireProjectMember(ctx context.Context, projectID int) error {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return ErrNotOwner
	}
	if user.IsOwner {
		return nil
	}

	count, err := authz.RunWithSystemBypass(ctx, "billing-project-membership-check", func(ctx context.Context) (int, error) {
		return r.client.UserProject.Query().
			Where(
				userproject.UserIDEQ(user.ID),
				userproject.ProjectIDEQ(projectID),
			).
			Count(ctx)
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotOwner
	}

	return nil
}
