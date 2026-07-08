package biz

import (
	"fmt"

	"github.com/shopspring/decimal"
)

const billingMicrosScale int32 = 6

var billingMicrosUnit = decimal.NewFromInt(1_000_000)

func decimalToMicros(amount decimal.Decimal) (int64, error) {
	if amount.IsNegative() {
		return 0, fmt.Errorf("amount must be non-negative")
	}

	scaled := amount.Mul(billingMicrosUnit)
	if !scaled.Equal(scaled.Truncate(0)) {
		return 0, fmt.Errorf("amount %s has more than %d decimal places", amount.String(), billingMicrosScale)
	}

	return scaled.IntPart(), nil
}

func microsToDecimal(micros int64) decimal.Decimal {
	return decimal.NewFromInt(micros).Div(billingMicrosUnit)
}
