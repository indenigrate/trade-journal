package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
)

// NewPool creates a pgxpool with connection retry and exponential backoff.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	var pool *pgxpool.Pool
	var err error

	for i := 0; i < 10; i++ {
		pool, err = pgxpool.New(ctx, databaseURL)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				log.Info().Msg("connected to PostgreSQL")
				return pool, nil
			}
			pool.Close()
		}

		wait := time.Duration(1<<uint(i)) * time.Second
		if wait > 30*time.Second {
			wait = 30 * time.Second
		}
		log.Warn().Err(err).Dur("retry_in", wait).Int("attempt", i+1).Msg("failed to connect to PostgreSQL")
		time.Sleep(wait)
	}

	return nil, fmt.Errorf("failed to connect to PostgreSQL after 10 attempts: %w", err)
}
