package gql

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/contexts"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/channel"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/objects"
	"github.com/looplj/axonhub/internal/server/biz"
)

func setupUpstreamAccountResolverTest(t *testing.T, name string) (*mutationResolver, *queryResolver, context.Context, *ent.Client, *ent.Channel) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:"+name+"?mode=memory&_fk=1")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))
	accountSvc := biz.NewUpstreamAccountService(biz.UpstreamAccountServiceParams{Ent: client})
	resolver := &Resolver{
		client:                 client,
		upstreamAccountService: accountSvc,
	}

	ch, err := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName("Resolver Pool Channel").
		SetCredentials(objects.ChannelCredentials{APIKey: "fallback-key"}).
		SetSupportedModels([]string{"gpt-test"}).
		SetDefaultTestModel("gpt-test").
		SetStatus(channel.StatusEnabled).
		Save(ctx)
	require.NoError(t, err)

	return &mutationResolver{resolver}, &queryResolver{resolver}, ctx, client, ch
}

func TestUpstreamAccountResolversRequireOwner(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, ch := setupUpstreamAccountResolverTest(t, "upstream_account_resolver_auth")
	defer client.Close()

	userCtx := contexts.WithUser(ctx, &ent.User{ID: 1, IsOwner: false})

	_, err := queryResolver.UpstreamAccounts(userCtx, objects.GUID{Type: ent.TypeChannel, ID: ch.ID}, nil)
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.CreateUpstreamAccountPool(userCtx, CreateUpstreamAccountPoolInput{
		ChannelID: objects.GUID{Type: ent.TypeChannel, ID: ch.ID},
		Name:      "default",
	})
	require.True(t, errors.Is(err, ErrNotOwner))

	_, err = mutationResolver.CreateUpstreamAccount(userCtx, CreateUpstreamAccountInput{
		ChannelID: objects.GUID{Type: ent.TypeChannel, ID: ch.ID},
		Name:      "account",
		Credentials: &biz.UpstreamAccountCredentialsInput{
			APIKey: ptr("sk-secret"),
		},
	})
	require.True(t, errors.Is(err, ErrNotOwner))
}

func TestUpstreamAccountResolversDoNotExposePlaintextCredentials(t *testing.T) {
	mutationResolver, queryResolver, ctx, client, ch := setupUpstreamAccountResolverTest(t, "upstream_account_resolver_credentials")
	defer client.Close()

	ownerCtx := contexts.WithUser(ctx, &ent.User{ID: 1, IsOwner: true})
	account, err := mutationResolver.CreateUpstreamAccount(ownerCtx, CreateUpstreamAccountInput{
		ChannelID: objects.GUID{Type: ent.TypeChannel, ID: ch.ID},
		Name:      "account",
		Credentials: &biz.UpstreamAccountCredentialsInput{
			APIKey: ptr("sk-secret-value"),
		},
	})
	require.NoError(t, err)

	accounts, err := queryResolver.UpstreamAccounts(ownerCtx, objects.GUID{Type: ent.TypeChannel, ID: ch.ID}, nil)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, account.ID, accounts[0].ID)

	hasCredentials, err := (&upstreamAccountResolver{mutationResolver.Resolver}).HasCredentials(ownerCtx, accounts[0])
	require.NoError(t, err)
	require.True(t, hasCredentials)
	require.NotContains(t, entgraphqlSchemaForTest(), "type UpstreamAccount implements Node {\n  credentials")
}

func entgraphqlSchemaForTest() string {
	for _, source := range sources {
		if source.Name == "ent.graphql" {
			return source.Input
		}
	}
	return ""
}
