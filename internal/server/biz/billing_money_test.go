package biz

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestBillingMoneyConversion(t *testing.T) {
	t.Parallel()

	micros, err := decimalToMicros(decimal.RequireFromString("12.345678"))
	require.NoError(t, err)
	require.Equal(t, int64(12345678), micros)
	require.True(t, microsToDecimal(micros).Equal(decimal.RequireFromString("12.345678")))
}

func TestBillingMoneyConversionRejectsTooManyDecimals(t *testing.T) {
	t.Parallel()

	_, err := decimalToMicros(decimal.RequireFromString("0.0000001"))
	require.Error(t, err)
}

func TestBillingMoneyConversionRejectsNegative(t *testing.T) {
	t.Parallel()

	_, err := decimalToMicros(decimal.RequireFromString("-1"))
	require.Error(t, err)
}

func TestUsageMoneyConversionRoundsPositiveSubMicrosUp(t *testing.T) {
	t.Parallel()

	micros, err := usageAmountToMicros(decimal.RequireFromString("0.00000149"))
	require.NoError(t, err)
	require.Equal(t, int64(2), micros)
	require.True(t, roundUsageAmount(decimal.RequireFromString("0.00008192")).Equal(decimal.RequireFromString("0.000082")))
}
