package gql

import (
	"context"

	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
)

func requireOwner(ctx context.Context) error {
	_, err := requireOwnerUser(ctx)
	return err
}

func requireOwnerUser(ctx context.Context) (*ent.User, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil || !user.IsOwner {
		return nil, ErrNotOwner
	}

	return user, nil
}
