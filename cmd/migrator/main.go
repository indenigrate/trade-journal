package main

import (
	"context"
	"database/sql"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = zerolog.New(os.Stdout).With().Timestamp().Logger()

	dbURL := getEnv("DATABASE_URL", "postgres://nevup:nevuppass@postgres-ts:5432/nevup?sslmode=disable")

	// Retry database connection
	var db *sql.DB
	var err error
	for i := 0; i < 20; i++ {
		db, err = sql.Open("pgx", dbURL)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err = db.PingContext(ctx)
			cancel()
			if err == nil {
				break
			}
		}
		wait := time.Duration(1<<uint(i)) * time.Second
		if wait > 10*time.Second {
			wait = 10 * time.Second
		}
		log.Warn().Err(err).Dur("retry_in", wait).Int("attempt", i+1).Msg("waiting for PostgreSQL")
		time.Sleep(wait)
	}
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to PostgreSQL after retries")
	}
	defer db.Close()

	migrationsDir := getEnv("MIGRATIONS_DIR", "/app/migrations")

	log.Info().Str("dir", migrationsDir).Msg("running migrations")
	if err := goose.Up(db, migrationsDir); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}

	log.Info().Msg("migrations completed successfully")
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
