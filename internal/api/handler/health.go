package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/onesine/nevup-backend/internal/cache"
)

type HealthHandler struct {
	pool         *pgxpool.Pool
	streamClient *cache.StreamClient
}

func NewHealthHandler(pool *pgxpool.Pool, sc *cache.StreamClient) *HealthHandler {
	return &HealthHandler{pool: pool, streamClient: sc}
}

// Check handles GET /health — no auth required.
func (h *HealthHandler) Check(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	dbStatus := "connected"
	if err := h.pool.Ping(ctx); err != nil {
		dbStatus = "disconnected"
	}

	queueLag := h.getQueueLag(ctx)

	status := "ok"
	if dbStatus == "disconnected" {
		status = "degraded"
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       status,
		"dbConnection": dbStatus,
		"queueLag":     queueLag,
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *HealthHandler) getQueueLag(ctx context.Context) int64 {
	info, err := h.streamClient.InfoStream(ctx, "trades:events")
	if err != nil {
		return 0
	}

	// Parse the last entry ID timestamp (Redis stream IDs are timestamp-sequence)
	if info.LastEntry.ID != "" {
		// Stream ID format: "timestamp-sequence"
		var ts int64
		fmt_parse := info.LastEntry.ID
		for i, c := range fmt_parse {
			if c == '-' {
				// Parse timestamp part
				for _, ch := range fmt_parse[:i] {
					ts = ts*10 + int64(ch-'0')
				}
				break
			}
		}
		if ts > 0 {
			now := time.Now().UnixMilli()
			lag := now - ts
			if lag < 0 {
				lag = 0
			}
			return lag
		}
	}
	return 0
}
