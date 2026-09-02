package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/shrimp-music/backend/db"
)

func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		rawToken := strings.TrimPrefix(authHeader, "Bearer ")
		
		h := sha256.New()
		h.Write([]byte(rawToken))
		tokenHash := hex.EncodeToString(h.Sum(nil))

		var userID string
		err := db.DB.QueryRow("SELECT user_id FROM user_tokens WHERE token_hash = ? AND is_revoked = 0", tokenHash).Scan(&userID)
		if err != nil {
			http.Error(w, "Unauthorized or token revoked", http.StatusUnauthorized)
			return
		}

		// Asynchronous Debounced Update of last_used_at
		go func(tHash string) {
			// Update only if older than 1 hour
			query := `UPDATE user_tokens SET last_used_at = CURRENT_TIMESTAMP WHERE token_hash = ? AND last_used_at < datetime('now', '-1 hour')`
			db.DB.Exec(query, tHash)
		}(tokenHash)

		ctx := context.WithValue(r.Context(), "user_id", userID)
		ctx = context.WithValue(ctx, "token_hash", tokenHash)
		
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

