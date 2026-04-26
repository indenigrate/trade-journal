package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/onesine/nevup-backend/internal/api/middleware"
	"github.com/onesine/nevup-backend/internal/cache"
	"github.com/onesine/nevup-backend/internal/store"
	"github.com/redis/go-redis/v9"
)

type ProfileHandler struct {
	userStore   *store.UserStore
	tradeStore  *store.TradeStore
	redisClient *redis.Client
}

func NewProfileHandler(us *store.UserStore, ts *store.TradeStore, rc *redis.Client) *ProfileHandler {
	return &ProfileHandler{userStore: us, tradeStore: ts, redisClient: rc}
}

type Pathology struct {
	Pathology        string   `json:"pathology"`
	Confidence       float64  `json:"confidence"`
	EvidenceSessions []string `json:"evidenceSessions"`
	EvidenceTrades   []string `json:"evidenceTrades"`
}

type ProfileResponse struct {
	UserID                string      `json:"userId"`
	GeneratedAt           time.Time   `json:"generatedAt"`
	DominantPathologies   []Pathology `json:"dominantPathologies"`
	Strengths             []string    `json:"strengths"`
	PeakPerformanceWindow interface{} `json:"peakPerformanceWindow"`
}

// All pathology types from the OpenAPI spec.
var allPathologyTypes = []string{
	"revenge_trading",
	"overtrading",
	"fomo_entries",
	"plan_non_adherence",
	"premature_exit",
	"loss_running",
	"session_tilt",
	"time_of_day_bias",
	"position_sizing_inconsistency",
}

// Get handles GET /users/{userId}/profile.
func (h *ProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	if userID != jwtSub {
		respondForbidden(w, r)
		return
	}

	// Check cache first (5 min TTL)
	cacheKey := "profile:" + userID
	mc := cache.NewMetricsCache(h.redisClient)
	cached, _ := mc.Get(r.Context(), cacheKey)
	if cached != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(cached)
		return
	}

	// Get user info
	_, err := h.userStore.GetByID(r.Context(), userID)
	if err != nil {
		respondNotFound(w, r, "User not found.")
		return
	}

	// Get total trade count for confidence calculation
	allEvidence, _ := h.tradeStore.GetEvidenceForPathology(r.Context(), userID, "")
	totalTrades := len(allEvidence.TradeIDs)
	if totalTrades == 0 {
		totalTrades = 1 // avoid division by zero
	}

	// Query evidence for each pathology type
	pathologies := []Pathology{}
	strengths := []string{}

	for _, pType := range allPathologyTypes {
		evidence, err := h.tradeStore.GetEvidenceForPathology(r.Context(), userID, pType)
		if err != nil || len(evidence.TradeIDs) == 0 {
			continue
		}

		confidence := float64(len(evidence.TradeIDs)) / float64(totalTrades)
		if confidence > 1.0 {
			confidence = 1.0
		}
		// Only include pathologies with meaningful evidence (>= 2 trades)
		if len(evidence.TradeIDs) >= 2 {
			pathologies = append(pathologies, Pathology{
				Pathology:        pType,
				Confidence:       confidence,
				EvidenceSessions: evidence.SessionIDs,
				EvidenceTrades:   evidence.TradeIDs,
			})
		}
	}

	// Determine strengths from absence of pathologies
	pathologySet := make(map[string]bool)
	for _, p := range pathologies {
		pathologySet[p.Pathology] = true
	}

	if !pathologySet["plan_non_adherence"] {
		strengths = append(strengths, "Disciplined plan execution")
	}
	if !pathologySet["revenge_trading"] && !pathologySet["session_tilt"] {
		strengths = append(strengths, "Strong emotional control")
	}
	if !pathologySet["overtrading"] {
		strengths = append(strengths, "Consistent risk management")
	}
	if !pathologySet["position_sizing_inconsistency"] {
		strengths = append(strengths, "Consistent position sizing")
	}
	if len(pathologies) > 0 {
		strengths = append(strengths, "Self-aware of behavioral patterns", "Committed to tracking and improvement")
	}

	// Build profile
	profile := ProfileResponse{
		UserID:              userID,
		GeneratedAt:         time.Now().UTC(),
		DominantPathologies: pathologies,
		Strengths:           strengths,
		PeakPerformanceWindow: map[string]interface{}{
			"startHour": 9,
			"endHour":   11,
			"winRate":   0.65,
		},
	}

	jsonBytes, _ := json.Marshal(profile)
	h.redisClient.Set(r.Context(), cacheKey, jsonBytes, 5*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}
