package biz

import (
	"errors"

	"github.com/looplj/axonhub/llm/transformer"
)

var (
	ErrInvalidJWT             = errors.New("invalid jwt token")
	ErrInvalidToken           = errors.New("invalid token")
	ErrInvalidAPIKey          = errors.New("invalid api key")
	ErrInvalidPassword        = errors.New("invalid password")
	ErrInvalidModel           = transformer.ErrInvalidModel
	ErrInternal               = errors.New("server internal error, please try again later")
	ErrAPIKeyOwnerRequired    = errors.New("owner api key is required")
	ErrServiceAccountRequired = errors.New("service account api key required")
	ErrAPIKeyScopeRequired    = errors.New("api key missing required scope")
	ErrAPIKeyNameRequired     = errors.New("api key name is required")
	ErrSystemNotInitialized   = errors.New("system not initialized")
	ErrOIDCLoginRequired      = errors.New("OIDC user without password, please login via OIDC or set a password")
	ErrProjectNotFound        = errors.New("project not found")
	ErrBillingAccountNotFound = errors.New("billing account not found")
	ErrBillingAccountFrozen   = errors.New("billing account is frozen")
	ErrBillingAccountClosed   = errors.New("billing account is closed")
	ErrBillingPriceNotFound   = errors.New("billing price not found")
	ErrInsufficientBalance    = errors.New("insufficient billing balance")
	ErrPaymentOrderNotFound   = errors.New("payment order not found")
	ErrPaymentOrderNotPayable = errors.New("payment order is not payable")
)
