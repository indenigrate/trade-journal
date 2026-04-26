package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/onesine/nevup-backend/internal/api/middleware"
	"github.com/onesine/nevup-backend/internal/domain"
	"github.com/onesine/nevup-backend/internal/store"
)

type SessionHandler struct {
	sessionStore *store.SessionStore
}

func NewSessionHandler(ss *store.SessionStore) *SessionHandler {
	return &SessionHandler{sessionStore: ss}
}

// Get handles GET /sessions/{sessionId}.
func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	summary, err := h.sessionStore.GetWithTrades(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondNotFound(w, r, "Session not found.")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch session")
		return
	}

	if summary.UserID != jwtSub {
		respondForbidden(w, r)
		return
	}

	respondJSON(w, http.StatusOK, summary)
}

// DebriefInput is the request body for POST /sessions/:id/debrief.
type DebriefInput struct {
	OverallMood         string  `json:"overallMood"`
	KeyMistake          *string `json:"keyMistake"`
	KeyLesson           *string `json:"keyLesson"`
	PlanAdherenceRating *int    `json:"planAdherenceRating"`
	WillReviewTomorrow  bool    `json:"willReviewTomorrow"`
}

// Debrief handles POST /sessions/{sessionId}/debrief.
func (h *SessionHandler) Debrief(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	// Check session ownership
	sess, err := h.sessionStore.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondNotFound(w, r, "Session not found.")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch session")
		return
	}
	if sess.UserID != jwtSub {
		respondForbidden(w, r)
		return
	}

	var input DebriefInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondBadRequest(w, r, "Invalid request body")
		return
	}

	if !domain.IsValidEmotionalState(input.OverallMood) {
		respondBadRequest(w, r, "Invalid overallMood")
		return
	}

	debrief := store.Debrief{
		OverallMood:         input.OverallMood,
		KeyMistake:          input.KeyMistake,
		KeyLesson:           input.KeyLesson,
		PlanAdherenceRating: input.PlanAdherenceRating,
		WillReviewTomorrow:  input.WillReviewTomorrow,
	}

	result, err := h.sessionStore.InsertDebrief(r.Context(), sessionID, debrief)
	if err != nil {
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to save debrief")
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"debriefId": result.DebriefID,
		"sessionId": result.SessionID,
		"savedAt":   result.SavedAt,
	})
}

// Coaching handles GET /sessions/{sessionId}/coaching (SSE).
func (h *SessionHandler) Coaching(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")
	jwtSub := middleware.UserIDFromContext(r.Context())

	session, err := h.sessionStore.GetSession(r.Context(), sessionID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			respondNotFound(w, r, "Session not found.")
			return
		}
		respondError(w, r, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to fetch session")
		return
	}

	if session.UserID != jwtSub {
		respondForbidden(w, r)
		return
	}

	// Set SSE headers before any write
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		return
	}

	message := generateCoachingMessage(session)
	tokens := tokenize(message)

	for i, token := range tokens {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		fmt.Fprintf(w, "event: token\ndata: {\"token\": %q, \"index\": %d}\n\n", token, i)
		flusher.Flush()
		time.Sleep(30 * time.Millisecond)
	}

	fmt.Fprintf(w, "event: done\ndata: {\"fullMessage\": %q}\n\n", message)
	flusher.Flush()
}

func generateCoachingMessage(session store.Session) string {
	notes := ""
	if session.Notes != nil {
		notes = *session.Notes
	}
	return fmt.Sprintf(
		"Great session review! Your trading session on %s showed commitment to self-improvement. %s Keep tracking your emotional state and plan adherence — consistency is the key to long-term profitability. Remember to review your trades tomorrow and identify any patterns in your decision-making process.",
		session.StartedAt.Format("January 2, 2006"),
		notes,
	)
}

func tokenize(message string) []string {
	return strings.Fields(message)
}
