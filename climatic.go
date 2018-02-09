package climatic

import (
	"math/big"
	"strconv"
)

// Ftos converts floats to strings.
func Ftos(v float64) string { return strconv.FormatFloat(v, 'E', -1, 64) }

// ParseFloat parses a float string.
func ParseFloat(v string) (*big.Float, error) {
	if v == "" {
		return nil, nil
	}
	f, _, err := big.ParseFloat(v, 10, 0, big.ToNearestEven)
	return f, err
}
