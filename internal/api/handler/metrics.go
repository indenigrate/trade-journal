package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/onesine/nevup-backend/internal/api/middleware"
	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/store"
)

type MetricsHandler struct {
	metricsStore *store.MetricsStore
	metricsCache *cache.MetricsCache
}

func NewMetricsHandler(ms *store.MetricsStore, mc *cache.MetricsCache) *MetricsHandler {
	return &MetricsHandler{metricsStore: ms, metricsCache: mc}
}

// Get handles GET /users/{userId}/metrics.
func (h *MetricsHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	// Tenancy check
	if userID != jwtSub {
		respondForbidden(w, r)
		return
	}

	// Parse query params
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	granularity := r.URL.Query().Get("granularity")

	if fromStr == "" || toStr == "" || granularity == "" {
		respondBadRequest(w, r, "from, to, and granularity query parameters are required")
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		respondBadRequest(w, r, "Invalid 'from' format, expected ISO-8601")
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		respondBadRequest(w, r, "Invalid 'to' format, expected ISO-8601")
		return
	}

	var interval string
	switch granularity {
	case "hourly":
		interval = "1 hour"
	case "daily":
		interval = "1 day"
	case "rolling30d":
		interval = "30 days"
	default:
		respondBadRequest(w, r, "granularity must be hourly, daily, or rolling30d")
		return
	}

	// Cache-aside: check Redis first
	cacheKey := fmt.Sprintf("metrics:%s:%s:%s:%s", userID, fromStr, toStr, granularity)
	cached, err := h.metricsCache.Get(r.Context(), cacheKey)
	if err == nil && cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cached)
		return
	}

	// Cache miss: query TimescaleDB
	buckets, err := h.metricsStore.QueryRange(r.Context(), userID, from, to, interval)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query metrics")
		return
	}

	// Query emotion win rates
	emotionRates, err := h.metricsStore.QueryEmotionWinRates(r.Context(), userID, from, to)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to query emotion win rates")
		return
	}

	// Count revenge trades and overtrading events
	revengeCount, _ := h.metricsStore.CountRevengeTrades(r.Context(), userID, from, to)
	overtradingCount, _ := h.metricsStore.CountOvertradingEvents(r.Context(), userID, from, to)

	// Compute aggregates
	var totalPlanAdherence float64
	var planAdherenceCount int
	var totalTiltIndex float64
	var tiltCount int

	type TimeseriesPoint struct {
		Bucket           time.Time `json:"bucket"`
		TradeCount       int       `json:"tradeCount"`
		WinRate          float64   `json:"winRate"`
		PnL              float64   `json:"pnl"`
		AvgPlanAdherence float64   `json:"avgPlanAdherence"`
	}

	timeseries := make([]TimeseriesPoint, 0, len(buckets))

	for _, b := range buckets {
		winRate := 0.0
		if b.TradeCount > 0 {
			winRate = float64(b.WinCount) / float64(b.TradeCount)
		}
		avgPA := 0.0
		if b.AvgPlanAdherence != nil {
			avgPA = *b.AvgPlanAdherence
			totalPlanAdherence += avgPA
			planAdherenceCount++
		}
		if b.SessionTiltIndex != nil {
			totalTiltIndex += *b.SessionTiltIndex
			tiltCount++
		}

		pnlFloat, _ := b.PnL.Float64()
		timeseries = append(timeseries, TimeseriesPoint{
			Bucket:           b.Bucket,
			TradeCount:       b.TradeCount,
			WinRate:          winRate,
			PnL:              pnlFloat,
			AvgPlanAdherence: avgPA,
		})
	}

	avgPlanAdherenceScore := 0.0
	if planAdherenceCount > 0 {
		avgPlanAdherenceScore = totalPlanAdherence / float64(planAdherenceCount)
	}
	avgSessionTilt := 0.0
	if tiltCount > 0 {
		avgSessionTilt = totalTiltIndex / float64(tiltCount)
	}

	// Build emotion win rate map
	emotionMap := make(map[string]map[string]interface{})
	for _, er := range emotionRates {
		emotionMap[er.EmotionalState] = map[string]interface{}{
			"wins":    er.Wins,
			"losses":  er.Losses,
			"winRate": er.WinRate,
		}
	}

	response := map[string]interface{}{
		"userId":                  userID,
		"granularity":             granularity,
		"from":                    fromStr,
		"to":                      toStr,
		"planAdherenceScore":      avgPlanAdherenceScore,
		"sessionTiltIndex":        avgSessionTilt,
		"winRateByEmotionalState": emotionMap,
		"revengeTrades":           revengeCount,
		"overtradingEvents":       overtradingCount,
		"timeseries":              timeseries,
	}

	// Write to cache
	jsonBytes, _ := json.Marshal(response)
	go h.metricsCache.Set(r.Context(), cacheKey, jsonBytes)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}
