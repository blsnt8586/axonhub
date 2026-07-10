package log

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type inlineSecret struct{}

func (inlineSecret) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	encoder.AddString("password", "stage23-inline-password")
	encoder.AddString("safe_inline", "stage23-safe-inline")
	return nil
}

func TestRedactString(t *testing.T) {
	secrets := []string{
		"stage23-bearer-token",
		"sk-stage23-api-key",
		"stage23-password",
		"stage23-dsn-password",
		"stage23-json-secret",
	}

	input := strings.Join([]string{
		"Authorization: Bearer stage23-bearer-token",
		"api_key=sk-stage23-api-key",
		"password=stage23-password",
		"postgres://axonhub:stage23-dsn-password@postgres:5432/axonhub",
		`{"secret":"stage23-json-secret","safe":"visible"}`,
	}, " ")

	redacted := RedactString(input)
	for _, secret := range secrets {
		require.NotContains(t, redacted, secret)
	}
	require.Contains(t, redacted, RedactedValue)
	require.Contains(t, redacted, `"safe":"visible"`)
}

func TestRedactingCoreSanitizesMessagesAndStructuredFields(t *testing.T) {
	var output bytes.Buffer
	encoder := zapcore.NewJSONEncoder(zapcore.EncoderConfig{
		MessageKey:  "message",
		LevelKey:    "level",
		EncodeLevel: zapcore.LowercaseLevelEncoder,
	})
	core := newRedactingCore(zapcore.NewCore(encoder, zapcore.AddSync(&output), zapcore.DebugLevel))
	logger := zap.New(core).With(zap.String("authorization", "Bearer stage23-with-field-token"))

	logger.Debug(
		"request failed with sk-stage23-message-key",
		zap.Any("headers", map[string][]string{
			"Authorization": {"Bearer stage23-header-token"},
			"Cookie":        {"session=stage23-cookie"},
			"X-Trace-ID":    {"stage23-safe-trace"},
		}),
		zap.Any("variables", map[string]any{
			"password": "stage23-variable-password",
			"input": map[string]any{
				"key":  "stage23-provider-key",
				"name": "stage23-safe-provider",
			},
		}),
		zap.String("dsn_message", "postgres://axonhub:stage23-dsn-password@postgres:5432/axonhub"),
		zap.ByteString("byte_body", []byte("password=stage23-byte-password")),
		zap.Binary("binary_body", []byte("secret=stage23-binary-secret")),
		zap.Error(errors.New("upstream rejected api_key=sk-stage23-error-key")),
		zap.Inline(inlineSecret{}),
	)

	logged := output.String()
	for _, secret := range []string{
		"stage23-with-field-token",
		"sk-stage23-message-key",
		"stage23-header-token",
		"stage23-cookie",
		"stage23-variable-password",
		"stage23-provider-key",
		"stage23-dsn-password",
		"stage23-byte-password",
		"stage23-binary-secret",
		"sk-stage23-error-key",
		"stage23-inline-password",
	} {
		require.NotContains(t, logged, secret)
	}

	require.Contains(t, logged, RedactedValue)
	require.Contains(t, logged, "stage23-safe-trace")
	require.Contains(t, logged, "stage23-safe-provider")
	require.Contains(t, logged, "stage23-safe-inline")
}

func TestSensitiveFieldNameNormalization(t *testing.T) {
	for _, name := range []string{
		"Authorization",
		"proxy-authorization",
		"X-API-Key",
		"x_goog_api_key",
		"payment.secret-key",
		"refresh_token",
		"DB_DSN",
		"upstream_credentials",
		"provider_key",
		"db.read_replica.read_dsn",
	} {
		require.True(t, isSensitiveFieldName(name), name)
	}

	for _, name := range []string{"api_key_name", "token_count", "project_id", "safe"} {
		require.False(t, isSensitiveFieldName(name), name)
	}
}
