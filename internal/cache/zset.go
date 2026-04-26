package cache

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// ZSetClient wraps Redis ZSET operations for the overtrading sliding window.
type ZSetClient struct {
	client *redis.Client
}

func NewZSetClient(client *redis.Client) *ZSetClient {
	return &ZSetClient{client: client}
}

// AddNX adds a member to a ZSET with NX flag (don't update existing scores).
func (z *ZSetClient) AddNX(ctx context.Context, key string, score float64, member string) error {
	return z.client.ZAddNX(ctx, key, redis.Z{
		Score:  score,
		Member: member,
	}).Err()
}

// RemoveByScoreRange removes members with scores outside the sliding window.
func (z *ZSetClient) RemoveByScoreRange(ctx context.Context, key string, min, max string) error {
	return z.client.ZRemRangeByScore(ctx, key, min, max).Err()
}

// Count returns the number of members in a score range.
func (z *ZSetClient) Count(ctx context.Context, key string, min, max string) (int64, error) {
	return z.client.ZCount(ctx, key, min, max).Result()
}

// OvertradingCheck performs the full sliding window check in a pipeline.
// Returns the count of trades in the window.
func (z *ZSetClient) OvertradingCheck(ctx context.Context, key string, nowMs int64, tradeID string, windowMs int64) (int64, error) {
	pipe := z.client.Pipeline()

	pipe.ZAddNX(ctx, key, redis.Z{Score: float64(nowMs), Member: tradeID})
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", nowMs-windowMs))
	countCmd := pipe.ZCount(ctx, key, fmt.Sprintf("%d", nowMs-windowMs), "+inf")

	_, err := pipe.Exec(ctx)
	if err != nil {
		return 0, err
	}

	return countCmd.Val(), nil
}
