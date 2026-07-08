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
