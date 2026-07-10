package authz

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithUserAPIKeyDelegationRequiresMatchingUser(t *testing.T) {
	ctx := NewUserContext(context.Background(), 7)

	delegated, err := WithUserAPIKeyDelegation(ctx, 7, 11, 13)
	require.NoError(t, err)
	principal, ok := GetPrincipal(delegated)
	require.True(t, ok)
	require.Equal(t, PrincipalTypeAPIKey, principal.Type)
	require.Equal(t, 11, *principal.APIKeyID)
	require.Equal(t, 13, *principal.ProjectID)

	_, err = WithUserAPIKeyDelegation(ctx, 8, 11, 13)
	require.Error(t, err)
	_, err = WithUserAPIKeyDelegation(context.Background(), 7, 11, 13)
	require.Error(t, err)
}
