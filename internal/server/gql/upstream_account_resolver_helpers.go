package gql

import (
	"github.com/looplj/axonhub/internal/ent/upstreamaccount"
	"github.com/looplj/axonhub/internal/ent/upstreamaccountpool"
	"github.com/looplj/axonhub/internal/objects"
)

func boolOrDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func intOrDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func floatOrDefault(value *float64, fallback float64) float64 {
	if value == nil {
		return fallback
	}
	return *value
}

func intPtrToInt64Ptr(value *int) *int64 {
	if value == nil {
		return nil
	}
	converted := int64(*value)
	return &converted
}

func intPtrToInt64OrDefault(value *int, fallback int64) int64 {
	if value == nil {
		return fallback
	}
	return int64(*value)
}

func guidIDPtr(value *objects.GUID) *int {
	if value == nil {
		return nil
	}
	return &value.ID
}

func upstreamAccountStatusOrDefault(value *upstreamaccount.Status) upstreamaccount.Status {
	if value == nil {
		return upstreamaccount.StatusActive
	}
	return *value
}

func upstreamAccountCredentialTypeOrDefault(value *upstreamaccount.CredentialType) upstreamaccount.CredentialType {
	if value == nil {
		return upstreamaccount.CredentialTypeAPIKey
	}
	return *value
}

func upstreamAccountPoolStatusOrDefault(value *upstreamaccountpool.Status) upstreamaccountpool.Status {
	if value == nil {
		return upstreamaccountpool.StatusEnabled
	}
	return *value
}
