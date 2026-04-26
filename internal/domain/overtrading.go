package domain

import "time"

const (
	// MaxTradesPerWindow is the threshold for overtrading detection.
	// More than 10 trades in a 30-minute window triggers an overtrading event.
	MaxTradesPerWindow = 10

	// WindowDuration is the sliding window for overtrading detection.
	WindowDuration = 30 * time.Minute

	// WindowDurationMs is the window duration in milliseconds (for Redis ZSET).
	WindowDurationMs = 1_800_000
)
