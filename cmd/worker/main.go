package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/pipeline"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := getEnv("DATABASE_URL", "postgres://nevup:nevuppass@postgres-ts:5432/nevup?sslmode=disable")
	redisAddr := getEnv("REDIS_URL", "redis:6379")

	// PostgreSQL pool
	pool, err := store.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL")
	}
	defer pool.Close()

	// Redis
	redisClient, err := cache.NewRedisClient(ctx, redisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	// Clients
	streamClient := cache.NewStreamClient(redisClient)
	zsetClient := cache.NewZSetClient(redisClient)
	metricsCache := cache.NewMetricsCache(redisClient)

	// Stores
	metricsStore := store.NewMetricsStore(pool)
	tradeStore := store.NewTradeStore(pool)

	// Pipeline worker
	worker := pipeline.NewWorker(streamClient, zsetClient, metricsStore, tradeStore, metricsCache)

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutting down pipeline worker...")
		cancel()
	}()

	if err := worker.Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("worker error")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
