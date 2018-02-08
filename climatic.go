package climatic

import "strconv"

// Ftos converts floats to strings.
func Ftos(v float64) string { return strconv.FormatFloat(v, 'E', -1, 64) }
