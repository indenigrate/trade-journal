package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// StreamClient wraps Redis Stream operations.
type StreamClient struct {
	client *redis.Client
}

func NewStreamClient(client *redis.Client) *StreamClient {
	return &StreamClient{client: client}
}

// CreateGroup creates a consumer group on a stream. Ignores BUSYGROUP errors.
func (s *StreamClient) CreateGroup(ctx context.Context, stream, group, start string) error {
	err := s.client.XGroupCreateMkStream(ctx, stream, group, start).Err()
	if err != nil {
		// Ignore BUSYGROUP — group already exists
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return nil
		}
		return err
	}
	return nil
}

// Add publishes an event to a stream.
func (s *StreamClient) Add(ctx context.Context, stream string, values map[string]interface{}) (string, error) {
	return s.client.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: values,
	}).Result()
}

// ReadGroup reads messages from a consumer group.
func (s *StreamClient) ReadGroup(ctx context.Context, group, consumer, stream string, count int64, block time.Duration) ([]redis.XStream, error) {
	return s.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    group,
		Consumer: consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
}

// Ack acknowledges messages in a consumer group.
func (s *StreamClient) Ack(ctx context.Context, stream, group string, ids ...string) error {
	return s.client.XAck(ctx, stream, group, ids...).Err()
}

// InfoStream returns stream info for health check.
func (s *StreamClient) InfoStream(ctx context.Context, stream string) (*redis.XInfoStream, error) {
	return s.client.XInfoStream(ctx, stream).Result()
}
