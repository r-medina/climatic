package climatic

import "math/big"

// Ftos converts floats to strings.
func Ftos(f *big.Float) string { return f.Text('f', 64) }

// ParseFloat parses a float string.
func ParseFloat(v string) (*big.Float, error) {
	if v == "" {
		return nil, nil
	}
	f, _, err := big.ParseFloat(v, 10, 512, big.ToNearestEven)
	return f, err
}
