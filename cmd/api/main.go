package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/onesine/nevup-backend/internal/api"
	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Structured JSON logging
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Config from env
	dbURL := getEnv("DATABASE_URL", "postgres://nevup:nevuppass@postgres-ts:5432/nevup?sslmode=disable")
	redisAddr := getEnv("REDIS_URL", "redis:6379")
	jwtSecret := getEnv("JWT_SECRET", "97791d4db2aa5f689c3cc39356ce35762f0a73aa70923039d8ef72a2840a1b02")
	port := getEnv("PORT", "8080")

	// PostgreSQL pool
	pool, err := store.NewPool(ctx, dbURL)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL")
	}
	defer pool.Close()

	// Redis client
	redisClient, err := cache.NewRedisClient(ctx, redisAddr)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to Redis")
	}
	defer redisClient.Close()

	// Cache + stream clients
	streamClient := cache.NewStreamClient(redisClient)
	zsetClient := cache.NewZSetClient(redisClient)
	metricsCache := cache.NewMetricsCache(redisClient)

	// Create consumer group (idempotent)
	_ = streamClient.CreateGroup(ctx, "trades:events", "nevup-analytics", "0")

	// Router
	router := api.NewRouter(log.Logger, jwtSecret, pool, redisClient, streamClient, zsetClient, metricsCache)

	// HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Info().Msg("shutting down HTTP server...")
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		srv.Shutdown(shutCtx)
		cancel()
	}()

	log.Info().Str("port", port).Msg("API server starting")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("server error")
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
