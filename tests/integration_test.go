package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var (
	baseURL   string
	jwtSecret string
)

func TestMain(m *testing.M) {
	baseURL = getEnv("BASE_URL", "http://localhost:8080")
	jwtSecret = getEnv("JWT_SECRET", "97791d4db2aa5f689c3cc39356ce35762f0a73aa70923039d8ef72a2840a1b02")

	// Wait for the API to be ready
	for i := 0; i < 30; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		time.Sleep(1 * time.Second)
	}

	os.Exit(m.Run())
}

func makeJWT(sub string, expiresIn time.Duration) string {
	claims := jwt.MapClaims{
		"sub":  sub,
		"role": "trader",
		"iat":  time.Now().Unix(),
		"exp":  time.Now().Add(expiresIn).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(jwtSecret))
	return s
}

func TestPostTradesIdempotency(t *testing.T) {
	userID := "f412f236-4edc-47a2-8f54-8763a6ed2ce8" // Alex Mercer
	tok := makeJWT(userID, 1*time.Hour)
	tradeID := uuid.New().String()

	body := map[string]interface{}{
		"tradeId":    tradeID,
		"userId":     userID,
		"sessionId":  "4f39c2ea-8687-41f7-85a0-1fafd3e976df",
		"asset":      "AAPL",
		"assetClass": "equity",
		"direction":  "long",
		"entryPrice": 150.50,
		"exitPrice":  155.00,
		"quantity":   10,
		"entryAt":    "2025-03-01T10:00:00Z",
		"exitAt":     "2025-03-01T11:00:00Z",
		"status":     "closed",
	}

	// First POST
	resp1 := doPost(t, baseURL+"/trades", tok, body)
	require.Equal(t, 200, resp1.StatusCode)

	var result1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&result1)
	resp1.Body.Close()

	// Second POST — same tradeId
	resp2 := doPost(t, baseURL+"/trades", tok, body)
	require.Equal(t, 200, resp2.StatusCode)

	var result2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&result2)
	resp2.Body.Close()

	// createdAt must be identical (idempotent)
	require.Equal(t, result1["createdAt"], result2["createdAt"],
		"createdAt must be identical for idempotent POST")
}

func TestCrossTenantReturns403(t *testing.T) {
	// JWT for Alex Mercer
	alexToken := makeJWT("f412f236-4edc-47a2-8f54-8763a6ed2ce8", 1*time.Hour)

	// Request Jordan Lee's metrics
	url := fmt.Sprintf("%s/users/%s/metrics?from=2025-01-01T00:00:00Z&to=2025-03-01T00:00:00Z&granularity=daily",
		baseURL, "fcd434aa-2201-4060-aeb2-f44c77aa0683")

	resp := doGet(t, url, alexToken)
	require.Equal(t, 403, resp.StatusCode)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	require.Equal(t, "FORBIDDEN", body["error"])
	require.NotEmpty(t, body["traceId"], "403 response must include traceId")
}

func TestExpiredTokenReturns401(t *testing.T) {
	// JWT that expired 1 hour ago
	expiredToken := makeJWT("f412f236-4edc-47a2-8f54-8763a6ed2ce8", -1*time.Hour)

	resp := doGet(t, baseURL+"/trades/some-trade-id", expiredToken)
	require.Equal(t, 401, resp.StatusCode)

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	resp.Body.Close()

	require.NotEmpty(t, body["traceId"], "401 response must include traceId")
}

func doPost(t *testing.T, url, token string, body interface{}) *http.Response {
	t.Helper()
	jsonBody, err := json.Marshal(body)
	require.NoError(t, err)

	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func doGet(t *testing.T, url, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest("GET", url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Verify unused body
var _ = io.ReadAll
