package log

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const RedactedValue = "[REDACTED]"

var sensitiveFieldNames = map[string]struct{}{
	"accesstoken":        {},
	"apikey":             {},
	"authorization":      {},
	"clientsecret":       {},
	"cookie":             {},
	"credential":         {},
	"credentials":        {},
	"databaseurl":        {},
	"dbdsn":              {},
	"dsn":                {},
	"idtoken":            {},
	"key":                {},
	"password":           {},
	"passwd":             {},
	"paymentsecretkey":   {},
	"providerkey":        {},
	"proxyauthorization": {},
	"pwd":                {},
	"readdsn":            {},
	"refreshtoken":       {},
	"redisurl":           {},
	"secret":             {},
	"secretkey":          {},
	"setcookie":          {},
	"token":              {},
	"xapikey":            {},
	"xgoogapikey":        {},
}

var sensitiveFieldSuffixes = []string{
	"accesstoken",
	"apikey",
	"authorization",
	"clientsecret",
	"cookie",
	"credential",
	"credentials",
	"dsn",
	"idtoken",
	"password",
	"passwd",
	"refreshtoken",
	"secret",
	"secretkey",
	"token",
}

var redactionPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`(?i)((?:postgres(?:ql)?|mysql|redis)://[^:/@\s]+:)([^@\s]+)(@)`),
		replacement: `${1}` + RedactedValue + `${3}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]{8,}`),
		replacement: `${1}` + RedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:authorization|proxy[_-]?authorization|x[_-]?api[_-]?key|x[_-]?goog[_-]?api[_-]?key|api[_-]?key|access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|payment[_-]?secret[_-]?key|secret[_-]?key|secret|password|passwd|pwd|cookie|set[_-]?cookie|dsn|read[_-]?dsn|database[_-]?url|redis[_-]?url|credential|credentials|token|key)["']?\s*[:=]\s*["'])([^"'\r\n]+)(["'])`),
		replacement: `${1}` + RedactedValue + `${3}`,
	},
	{
		pattern:     regexp.MustCompile(`(?i)((?:authorization|proxy[_-]?authorization|x[_-]?api[_-]?key|x[_-]?goog[_-]?api[_-]?key|api[_-]?key|access[_-]?token|refresh[_-]?token|id[_-]?token|client[_-]?secret|payment[_-]?secret[_-]?key|secret[_-]?key|secret|password|passwd|pwd|cookie|set[_-]?cookie|dsn|read[_-]?dsn|database[_-]?url|redis[_-]?url|credential|credentials|token|key)\s*[:=]\s*)([^\s,;&]+)`),
		replacement: `${1}` + RedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
		replacement: RedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{20,}\b`),
		replacement: RedactedValue,
	},
	{
		pattern:     regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
		replacement: RedactedValue,
	},
}

func RedactString(value string) string {
	for _, rule := range redactionPatterns {
		value = rule.pattern.ReplaceAllString(value, rule.replacement)
	}

	return value
}

func isSensitiveFieldName(name string) bool {
	var normalized strings.Builder
	normalized.Grow(len(name))

	for _, char := range strings.ToLower(name) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			normalized.WriteRune(char)
		}
	}

	fieldName := normalized.String()
	if _, ok := sensitiveFieldNames[fieldName]; ok {
		return true
	}

	for _, suffix := range sensitiveFieldSuffixes {
		if strings.HasSuffix(fieldName, suffix) {
			return true
		}
	}

	return false
}

func redactFields(fields []zap.Field) []zap.Field {
	if len(fields) == 0 {
		return fields
	}

	redacted := make([]zap.Field, 0, len(fields))
	for _, field := range fields {
		redacted = append(redacted, redactField(field))
	}

	return redacted
}

func redactField(field zap.Field) zap.Field {
	if field.Type != zapcore.NamespaceType && isSensitiveFieldName(field.Key) {
		return zap.String(field.Key, RedactedValue)
	}

	switch field.Type {
	case zapcore.StringType:
		field.String = RedactString(field.String)
		return field
	case zapcore.ByteStringType:
		if value, ok := field.Interface.([]byte); ok {
			return zap.ByteString(field.Key, []byte(RedactString(string(value))))
		}
	case zapcore.BinaryType:
		if value, ok := field.Interface.([]byte); ok {
			return zap.Binary(field.Key, []byte(RedactString(string(value))))
		}
	case zapcore.ReflectType:
		return zap.Reflect(field.Key, redactStructuredValue(field.Interface))
	case zapcore.ErrorType:
		if err, ok := field.Interface.(error); ok {
			return zap.String(field.Key, RedactString(err.Error()))
		}
	case zapcore.StringerType:
		if value, ok := field.Interface.(fmt.Stringer); ok {
			return zap.String(field.Key, RedactString(value.String()))
		}
	case zapcore.ObjectMarshalerType, zapcore.InlineMarshalerType:
		if value, ok := field.Interface.(zapcore.ObjectMarshaler); ok {
			marshaler := redactedObjectMarshaler{value: value}
			if field.Type == zapcore.InlineMarshalerType {
				return zap.Inline(marshaler)
			}
			return zap.Object(field.Key, marshaler)
		}
	}

	return field
}

type redactedObjectMarshaler struct {
	value zapcore.ObjectMarshaler
}

func (marshaler redactedObjectMarshaler) MarshalLogObject(encoder zapcore.ObjectEncoder) error {
	buffer := zapcore.NewMapObjectEncoder()
	if err := marshaler.value.MarshalLogObject(buffer); err != nil {
		return err
	}

	redacted, ok := redactDecodedValue(buffer.Fields).(map[string]any)
	if !ok {
		return nil
	}

	keys := make([]string, 0, len(redacted))
	for key := range redacted {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := encoder.AddReflected(key, redacted[key]); err != nil {
			return err
		}
	}

	return nil
}

func redactStructuredValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return RedactString(typed)
	case []byte:
		return RedactString(string(typed))
	case error:
		return RedactString(typed.Error())
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return RedactString(fmt.Sprint(value))
	}

	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return RedactString(string(encoded))
	}

	return redactDecodedValue(decoded)
}

func redactDecodedValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		redacted := make(map[string]any, len(typed))
		for key, item := range typed {
			if isSensitiveFieldName(key) {
				redacted[key] = RedactedValue
				continue
			}
			redacted[key] = redactDecodedValue(item)
		}
		return redacted
	case []any:
		redacted := make([]any, len(typed))
		for index, item := range typed {
			redacted[index] = redactDecodedValue(item)
		}
		return redacted
	case string:
		return RedactString(typed)
	default:
		return value
	}
}

type redactingCore struct {
	zapcore.Core
}

func newRedactingCore(core zapcore.Core) zapcore.Core {
	return &redactingCore{Core: core}
}

func (core *redactingCore) With(fields []zap.Field) zapcore.Core {
	return &redactingCore{Core: core.Core.With(redactFields(fields))}
}

func (core *redactingCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !core.Enabled(entry.Level) {
		return checked
	}

	return checked.AddCore(entry, core)
}

func (core *redactingCore) Write(entry zapcore.Entry, fields []zap.Field) error {
	entry.Message = RedactString(entry.Message)
	return core.Core.Write(entry, redactFields(fields))
}
