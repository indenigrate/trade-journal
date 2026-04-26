package domain

// RollingAvg computes the average of the last N ratings (up to 10).
// Returns 0.0 for an empty slice.
func RollingAvg(ratings []int) float64 {
	if len(ratings) == 0 {
		return 0.0
	}

	// Take only the last 10
	start := 0
	if len(ratings) > 10 {
		start = len(ratings) - 10
	}
	subset := ratings[start:]

	sum := 0
	for _, r := range subset {
		sum += r
	}
	return float64(sum) / float64(len(subset))
}
