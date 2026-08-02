package core

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const quantityScale int64 = 1_000_000

type Quantity int64

func QuantityFromFloat(value float64) (Quantity, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > float64(math.MaxInt64)/float64(quantityScale) {
		return 0, errors.New("quantity is outside the supported range")
	}
	return Quantity(math.Round(value * float64(quantityScale))), nil
}

func ParseQuantity(value string) (Quantity, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, errors.New("quantity is empty")
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse quantity: %w", err)
	}
	return QuantityFromFloat(parsed)
}

func (quantity Quantity) Float64() float64 { return float64(quantity) / float64(quantityScale) }

func (quantity Quantity) DatabaseString() string {
	value := strconv.FormatFloat(quantity.Float64(), 'f', 6, 64)
	value = strings.TrimRight(strings.TrimRight(value, "0"), ".")
	if value == "" {
		return "0"
	}
	return value
}

func CeilDiv(numerator, denominator Quantity) int64 {
	if numerator <= 0 || denominator <= 0 {
		return 0
	}
	return (int64(numerator) + int64(denominator) - 1) / int64(denominator)
}
