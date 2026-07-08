package gql

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPaymentProviderInstanceGraphQLSchemaDoesNotExposeConfig(t *testing.T) {
	t.Parallel()

	raw, err := os.ReadFile("ent.graphql")
	require.NoError(t, err)

	schema := string(raw)
	paymentProviderType := betweenRequired(t, schema, "type PaymentProviderInstance implements Node {", "type PaymentProviderInstanceConnection {")
	require.NotContains(t, paymentProviderType, "config:")

	paymentProviderWhere := betweenRequired(t, schema, "input PaymentProviderInstanceWhereInput {", "type Project implements Node {")
	require.NotContains(t, paymentProviderWhere, "config")
}

func betweenRequired(t *testing.T, value, start, end string) string {
	t.Helper()

	startIndex := strings.Index(value, start)
	require.NotEqual(t, -1, startIndex)

	remaining := value[startIndex:]
	endIndex := strings.Index(remaining, end)
	require.NotEqual(t, -1, endIndex)

	return remaining[:endIndex]
}
