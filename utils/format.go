package utils

import "fmt"

const (
	SunPerTRX = 1_000_000
)

// SunToTRX converts sun (smallest unit) to TRX.
func SunToTRX(sun int64) float64 {
	return float64(sun) / SunPerTRX
}

// FormatTRX returns a human-readable TRX string from sun value.
func FormatTRX(sun int64) string {
	return fmt.Sprintf("%.6f TRX", SunToTRX(sun))
}
