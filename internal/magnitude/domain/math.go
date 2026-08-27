package domain

import "math"

func Clamp(v, min, max float64) float64 {
	if math.IsNaN(v) {
		return min
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
func SafeLogAmplitude(amplitude float64) float64 {
	if amplitude <= 0 {
		return math.Inf(-1)
	}
	return math.Log10(amplitude)
}
