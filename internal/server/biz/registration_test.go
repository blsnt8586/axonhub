package biz

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/looplj/axonhub/internal/authz"
	"github.com/looplj/axonhub/internal/ent"
	"github.com/looplj/axonhub/internal/ent/apikey"
	"github.com/looplj/axonhub/internal/ent/enttest"
	"github.com/looplj/axonhub/internal/ent/ledgertransaction"
	"github.com/looplj/axonhub/internal/ent/user"
	"github.com/looplj/axonhub/internal/pkg/xcache"
)

func setupRegistrationServiceTest(t *testing.T) (*ent.Client, context.Context, *RegistrationService) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:registration?mode=memory&_fk=1")
	t.Cleanup(func() { _ = client.Close() })

	ctx := ent.NewContext(authz.WithTestBypass(context.Background()), client)
	cacheConfig := xcache.Config{Mode: xcache.ModeMemory}
	systemService := NewSystemService(SystemServiceParams{Ent: client, CacheConfig: cacheConfig})
	billingAccountService := NewBillingAccountService(BillingAccountServiceParams{Ent: client})
	ledgerService := NewLedgerService(LedgerServiceParams{Ent: client})
	billingAuditService := NewBillingAuditService(BillingAuditServiceParams{Ent: client})
	projectService := NewProjectService(ProjectServiceParams{Ent: client, CacheConfig: cacheConfig})
	apiKeyService := NewAPIKeyService(APIKeyServiceParams{
		Ent:            client,
		CacheConfig:    cacheConfig,
		ProjectService: projectService,
		KeyPrefix:      "ah",
	})
	t.Cleanup(apiKeyService.Stop)

	service := NewRegistrationService(RegistrationServiceParams{
		Ent:                   client,
		SystemService:         systemService,
		BillingAccountService: billingAccountService,
		LedgerService:         ledgerService,
		ProjectService:        projectService,
		APIKeyService:         apiKeyService,
		BillingAuditService:   billingAuditService,
	})

	return client, ctx, service
}

func TestRegistrationService_RegisterDisabledByDefault(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)

	_, err := service.Register(ctx, RegisterUserInput{
		Email:    "user@example.com",
		Password: "Password1",
	})
	require.ErrorIs(t, err, ErrRegistrationDisabled)
}

func TestRegistrationService_RegisterCreatesActivatedUserAndBillingAccount(t *testing.T) {
	client, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{Enabled: true}))

	result, err := service.Register(ctx, RegisterUserInput{
		Email:          " USER@example.com ",
		Password:       "Password1",
		PreferLanguage: "zh-CN",
	})
	require.NoError(t, err)
	require.NotNil(t, result.User)
	require.NotNil(t, result.BillingAccount)
	require.Equal(t, "user@example.com", result.User.Email)
	require.Equal(t, user.StatusActivated, result.User.Status)
	require.False(t, result.User.IsOwner)
	require.Empty(t, result.User.Scopes)
	require.Equal(t, "zh-CN", result.User.PreferLanguage)
	require.Equal(t, int64(0), result.BillingAccount.BalanceMicros)

	count, err := client.BillingAccountBinding.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestRegistrationService_RegisterDuplicateEmail(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{Enabled: true}))

	input := RegisterUserInput{Email: "duplicate@example.com", Password: "Password1"}
	_, err := service.Register(ctx, input)
	require.NoError(t, err)

	_, err = service.Register(ctx, input)
	require.ErrorIs(t, err, ErrRegistrationEmailExists)
}

func TestRegistrationService_RegisterRequireApprovalCreatesDeactivatedUser(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:         true,
		RequireApproval: true,
	}))

	result, err := service.Register(ctx, RegisterUserInput{
		Email:    "approval@example.com",
		Password: "Password1",
	})
	require.NoError(t, err)
	require.Equal(t, user.StatusDeactivated, result.User.Status)
}

func TestRegistrationService_RegisterRejectsWeakPassword(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{Enabled: true}))

	_, err := service.Register(ctx, RegisterUserInput{
		Email:    "weak@example.com",
		Password: "password",
	})
	require.ErrorIs(t, err, ErrRegistrationPasswordWeak)
}

func TestRegistrationService_RegisterSignupGrantPostsLedger(t *testing.T) {
	client, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:           true,
		SignupGrantAmount: "12.34",
	}))

	result, err := service.Register(ctx, RegisterUserInput{
		Email:    "grant@example.com",
		Password: "Password1",
	})
	require.NoError(t, err)
	require.Equal(t, int64(12_340_000), result.BillingAccount.BalanceMicros)

	txs, err := client.LedgerTransaction.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, ledgertransaction.TypeAdminAdjustment, txs[0].Type)
	require.Equal(t, "signup grant", txs[0].Memo)
}

func TestRegistrationService_RegisterCreatesDefaultProjectAndAPIKey(t *testing.T) {
	client, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:              true,
		CreateDefaultProject: true,
		CreateDefaultAPIKey:  true,
		DefaultProjectName:   "Starter",
		DefaultAPIKeyName:    "Starter Key",
	}))

	result, err := service.Register(ctx, RegisterUserInput{
		Email:    "project@example.com",
		Password: "Password1",
	})
	require.NoError(t, err)
	require.NotNil(t, result.Project)
	require.NotNil(t, result.APIKey)
	require.Equal(t, "Starter", result.Project.Name)
	require.Equal(t, "Starter Key", result.APIKey.Name)
	require.Equal(t, apikey.TypePersonal, result.APIKey.Type)
	require.Equal(t, result.User.ID, result.APIKey.UserID)
	require.Equal(t, result.Project.ID, result.APIKey.ProjectID)

	membershipCount, err := client.UserProject.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, membershipCount)
}

func TestRegistrationService_RegisterWritesAuditLog(t *testing.T) {
	client, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{Enabled: true}))

	result, err := service.Register(ctx, RegisterUserInput{
		Email:    "audit@example.com",
		Password: "Password1",
		ClientIP: "127.0.0.1",
	})
	require.NoError(t, err)

	logs, err := client.BillingAuditLog.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "registration.user_created", logs[0].Action)
	require.Equal(t, AuditTargetID(result.User.ID), logs[0].TargetID)
	require.NotNil(t, logs[0].TargetUserID)
	require.Equal(t, result.User.ID, *logs[0].TargetUserID)
}

func TestRegistrationService_RegisterRateLimited(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:                true,
		RateLimitWindowSeconds: 60,
		RateLimitMaxAttempts:   1,
	}))

	_, err := service.Register(ctx, RegisterUserInput{
		Email:    "limited@example.com",
		Password: "Password1",
		ClientIP: "127.0.0.1",
	})
	require.NoError(t, err)

	_, err = service.Register(ctx, RegisterUserInput{
		Email:    "limited@example.com",
		Password: "Password1",
		ClientIP: "127.0.0.1",
	})
	require.ErrorIs(t, err, ErrRegistrationRateLimited)
}

func TestRegistrationService_DefaultProjectNameAvoidsGlobalCollision(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)
	require.NoError(t, service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:              true,
		CreateDefaultProject: true,
		DefaultProjectName:   "Shared",
	}))

	first, err := service.Register(ctx, RegisterUserInput{Email: "first@example.com", Password: "Password1"})
	require.NoError(t, err)
	second, err := service.Register(ctx, RegisterUserInput{Email: "second@example.com", Password: "Password1"})
	require.NoError(t, err)

	require.Equal(t, "Shared", first.Project.Name)
	require.Equal(t, "Shared #"+fmt.Sprint(second.User.ID), second.Project.Name)
}

func TestRegistrationService_SetRegistrationSettingsRejectsInvalidGrant(t *testing.T) {
	_, ctx, service := setupRegistrationServiceTest(t)

	err := service.SetRegistrationSettings(ctx, RegistrationSettings{
		Enabled:           true,
		SignupGrantAmount: "-1",
	})
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrRegistrationDisabled))
}
