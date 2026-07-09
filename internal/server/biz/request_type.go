package biz

import (
	"strings"

	"github.com/looplj/axonhub/internal/ent/usagebillingrecord"
)

func inferCommercialRequestType(format string) string {
	normalized := strings.ToLower(format)
	switch {
	case strings.Contains(normalized, "image"):
		return string(usagebillingrecord.RequestTypeImage)
	case strings.Contains(normalized, "video"):
		return string(usagebillingrecord.RequestTypeVideo)
	case strings.Contains(normalized, "embedding"):
		return string(usagebillingrecord.RequestTypeEmbedding)
	case strings.Contains(normalized, "audio"), strings.Contains(normalized, "speech"), strings.Contains(normalized, "transcription"):
		return string(usagebillingrecord.RequestTypeAudio)
	case strings.Contains(normalized, "chat"), strings.Contains(normalized, "messages"), strings.Contains(normalized, "responses"):
		return string(usagebillingrecord.RequestTypeChat)
	default:
		return string(usagebillingrecord.RequestTypeOther)
	}
}
