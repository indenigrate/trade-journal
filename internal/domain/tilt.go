package domain

// SessionTiltIndex computes the ratio of loss-following trades to total trades.
// Returns 0.0 if totalCount is zero (avoids division by zero).
func SessionTiltIndex(lossFollowCount, totalCount int) float64 {
	if totalCount == 0 {
		return 0.0
	}
	return float64(lossFollowCount) / float64(totalCount)
}
