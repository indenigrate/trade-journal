package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware validates HS256 JWT tokens and injects sub into context.
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			traceID := TraceIDFromContext(r.Context())

			// 1. Extract Bearer token
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing Authorization header", traceID)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid Authorization header format", traceID)
				return
			}

			tokenString := parts[1]

			// 2. Parse + verify HS256 signature with validation
			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return []byte(secret), nil
			})

			if err != nil {
				if strings.Contains(err.Error(), "expired") {
					respondAuthError(w, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token has expired", traceID)
					return
				}
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token", traceID)
				return
			}

			if !token.Valid {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token", traceID)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token claims", traceID)
				return
			}

			// 3. Extract sub claim
			sub, ok := claims["sub"].(string)
			if !ok || sub == "" {
				respondAuthError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Missing sub claim", traceID)
				return
			}

			// 4. Inject sub into context
			ctx := context.WithValue(r.Context(), UserIDKey, sub)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func respondAuthError(w http.ResponseWriter, status int, errorCode, message, traceID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write([]byte(`{"error":"` + errorCode + `","message":"` + message + `","traceId":"` + traceID + `"}`))
}
