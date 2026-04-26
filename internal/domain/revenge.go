package domain

import "time"

// IsRevenge returns true if a trade opened within 90s (exclusive) of a losing close
// AND the emotional state is anxious or fearful.
func IsRevenge(prevExitAt, newEntryAt time.Time, emotionalState string) bool {
	gap := newEntryAt.Sub(prevExitAt)
	if gap >= 90*time.Second || gap < 0 {
		return false
	}
	return emotionalState == EmotionAnxious || emotionalState == EmotionFearful
}
