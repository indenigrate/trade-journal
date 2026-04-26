package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const MetricsCacheTTL = 5 * time.Minute

// MetricsCache handles cache-aside for behavioral metrics.
type MetricsCache struct {
	client *redis.Client
}

func NewMetricsCache(client *redis.Client) *MetricsCache {
	return &MetricsCache{client: client}
}

// Get retrieves cached metrics JSON bytes. Returns nil, nil if miss.
func (m *MetricsCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := m.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	return val, err
}

// Set stores metrics JSON bytes with TTL.
func (m *MetricsCache) Set(ctx context.Context, key string, data []byte) error {
	return m.client.Set(ctx, key, data, MetricsCacheTTL).Err()
}

// InvalidateUser deletes all cached metrics for a user using SCAN + DEL.
func (m *MetricsCache) InvalidateUser(ctx context.Context, userID string) error {
	pattern := "metrics:" + userID + ":*"
	iter := m.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		if err := m.client.Del(ctx, iter.Val()).Err(); err != nil {
			return err
		}
	}
	return iter.Err()
}

// SetLastLoss stores the last losing trade's exit time for revenge detection.
func (m *MetricsCache) SetLastLoss(ctx context.Context, userID string, exitAtUnix int64) error {
	key := "user:" + userID + ":last_loss"
	return m.client.Set(ctx, key, exitAtUnix, 90*time.Second).Err()
}

// GetLastLoss retrieves the last losing trade's exit time. Returns 0, nil on miss.
func (m *MetricsCache) GetLastLoss(ctx context.Context, userID string) (int64, error) {
	key := "user:" + userID + ":last_loss"
	val, err := m.client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}
