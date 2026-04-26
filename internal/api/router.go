package api

import (
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/onesine/nevup-backend/internal/api/handler"
	"github.com/onesine/nevup-backend/internal/api/middleware"
	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

// NewRouter creates the chi mux with all routes and middleware.
func NewRouter(
	logger zerolog.Logger,
	jwtSecret string,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	streamClient *cache.StreamClient,
	zsetClient *cache.ZSetClient,
	metricsCache *cache.MetricsCache,
) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Logger(logger))

	// Stores
	tradeStore := store.NewTradeStore(pool)
	sessionStore := store.NewSessionStore(pool)
	metricsStore := store.NewMetricsStore(pool)
	userStore := store.NewUserStore(pool)

	// Handlers
	tradeHandler := handler.NewTradeHandler(tradeStore, streamClient, zsetClient, metricsCache)
	sessionHandler := handler.NewSessionHandler(sessionStore)
	metricsHandler := handler.NewMetricsHandler(metricsStore, metricsCache)
	profileHandler := handler.NewProfileHandler(userStore, tradeStore, redisClient)
	healthHandler := handler.NewHealthHandler(pool, streamClient)

	// Health — no auth
	r.Get("/health", healthHandler.Check)

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.AuthMiddleware(jwtSecret))

		// Trades
		r.Post("/trades", tradeHandler.Create)
		r.Get("/trades/{tradeId}", tradeHandler.Get)

		// Sessions
		r.Get("/sessions/{sessionId}", sessionHandler.Get)
		r.Post("/sessions/{sessionId}/debrief", sessionHandler.Debrief)
		r.Get("/sessions/{sessionId}/coaching", sessionHandler.Coaching)

		// Users
		r.Get("/users/{userId}/metrics", metricsHandler.Get)
		r.Get("/users/{userId}/profile", profileHandler.Get)
	})

	return r
}
