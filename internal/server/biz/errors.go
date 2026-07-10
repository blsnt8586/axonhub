package biz

import (
	"errors"

	"github.com/looplj/axonhub/llm/transformer"
)

var (
	ErrInvalidJWT               = errors.New("invalid jwt token")
	ErrInvalidToken             = errors.New("invalid token")
	ErrInvalidAPIKey            = errors.New("invalid api key")
	ErrInvalidPassword          = errors.New("invalid password")
	ErrInvalidModel             = transformer.ErrInvalidModel
	ErrInternal                 = errors.New("server internal error, please try again later")
	ErrAPIKeyOwnerRequired      = errors.New("owner api key is required")
	ErrServiceAccountRequired   = errors.New("service account api key required")
	ErrAPIKeyScopeRequired      = errors.New("api key missing required scope")
	ErrAPIKeyNameRequired       = errors.New("api key name is required")
	ErrSystemNotInitialized     = errors.New("system not initialized")
	ErrOIDCLoginRequired        = errors.New("OIDC user without password, please login via OIDC or set a password")
	ErrRegistrationDisabled     = errors.New("registration is disabled")
	ErrRegistrationEmailExists  = errors.New("email already registered")
	ErrRegistrationInvalidEmail = errors.New("invalid registration email")
	ErrRegistrationPasswordWeak = errors.New(
		"password must be at least 8 characters and include uppercase, lowercase, and number",
	)
	ErrRegistrationRateLimited   = errors.New("too many registration attempts, please try again later")
	ErrProjectNotFound           = errors.New("project not found")
	ErrBillingAccountNotFound    = errors.New("billing account not found")
	ErrBillingAccountFrozen      = errors.New("billing account is frozen")
	ErrBillingAccountClosed      = errors.New("billing account is closed")
	ErrCommercialProfileDenied   = errors.New("commercial profile access denied")
	ErrWorkspaceProjectDenied    = errors.New("workspace project access denied")
	ErrWorkspaceCreationDisabled = errors.New("self-service workspace creation is disabled")
	ErrWorkspaceLimitReached     = errors.New("workspace limit reached")
	ErrWorkspaceInvalidName      = errors.New("workspace name must be between 1 and 80 characters")
	ErrUserAPIKeyDenied          = errors.New("user api key access denied")
	ErrUserAPIKeyInvalidInput    = errors.New("invalid user api key input")
	ErrBillingPriceNotFound      = errors.New("billing price not found")
	ErrInsufficientBalance       = errors.New("insufficient billing balance")
	ErrPaymentOrderNotFound      = errors.New("payment order not found")
	ErrPaymentOrderNotPayable    = errors.New("payment order is not payable")
	ErrRedeemCodeNotFound        = errors.New("redeem code not found")
	ErrRedeemCodeUsed            = errors.New("redeem code has already been used")
	ErrRedeemCodeDisabled        = errors.New("redeem code is disabled")
	ErrRedeemCodeExpired         = errors.New("redeem code is expired")
	ErrPromoCodeNotFound         = errors.New("promo code not found")
	ErrPromoCodeDisabled         = errors.New("promo code is disabled")
	ErrPromoCodeExpired          = errors.New("promo code is expired")
	ErrPromoCodeExhausted        = errors.New("promo code usage limit reached")
	ErrPromoCodeReused           = errors.New("promo code user limit reached")
	ErrPromoCodeScopeMismatch    = errors.New("promo code scope mismatch")
	ErrAffiliateInviteInvalid    = errors.New("affiliate invite code is invalid")
	ErrAffiliateSelfInvite       = errors.New("affiliate self-invite is not allowed")
	ErrAffiliateCircularInvite   = errors.New("affiliate circular invitation is not allowed")
	ErrAffiliateAlreadyBound     = errors.New("affiliate invitation already bound")
	ErrAffiliateRebateFrozen     = errors.New("affiliate rebate is still frozen")
)
