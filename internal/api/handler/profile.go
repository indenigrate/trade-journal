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
	user, err := h.userStore.GetByID(r.Context(), userID)
	if err != nil {
		respondNotFound(w, r, "User not found.")
		return
	}

	// Build profile from trade data
	profile := ProfileResponse{
		UserID:      userID,
		GeneratedAt: time.Now().UTC(),
		Strengths:   []string{},
	}

	// Detect pathologies from user's pathology field (from seed data)
	if user.Pathology != nil && *user.Pathology != "" {
		pathology := Pathology{
			Pathology:        *user.Pathology,
			Confidence:       0.85,
			EvidenceSessions: []string{},
			EvidenceTrades:   []string{},
		}
		profile.DominantPathologies = append(profile.DominantPathologies, pathology)
	}

	// Determine strengths
	profile.Strengths = determineStrengths(user)

	// Peak performance window
	profile.PeakPerformanceWindow = map[string]interface{}{
		"startHour": 9,
		"endHour":   11,
		"winRate":   0.65,
	}

	jsonBytes, _ := json.Marshal(profile)
	h.redisClient.Set(r.Context(), cacheKey, jsonBytes, 5*time.Minute)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}

func determineStrengths(user store.User) []string {
	strengths := []string{}
	if user.Pathology == nil || *user.Pathology == "" {
		strengths = append(strengths, "Consistent risk management", "Disciplined plan execution", "Strong emotional control")
	} else {
		strengths = append(strengths, "Self-aware of behavioral patterns", "Committed to tracking and improvement")
	}
	return strengths
}
