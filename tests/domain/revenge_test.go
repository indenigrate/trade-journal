package domain_test

import (
	"testing"
	"time"

	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestIsRevenge(t *testing.T) {
	base := time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		gap     time.Duration
		emotion string
		want    bool
	}{
		{"within 90s + anxious", 89 * time.Second, "anxious", true},
		{"within 90s + fearful", 45 * time.Second, "fearful", true},
		{"exactly 90s + anxious", 90 * time.Second, "anxious", false},
		{"over 90s + anxious", 91 * time.Second, "anxious", false},
		{"within 90s + calm", 60 * time.Second, "calm", false},
		{"within 90s + greedy", 60 * time.Second, "greedy", false},
		{"within 90s + neutral", 60 * time.Second, "neutral", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.IsRevenge(base, base.Add(tc.gap), tc.emotion)
			require.Equal(t, tc.want, got)
		})
	}
}
