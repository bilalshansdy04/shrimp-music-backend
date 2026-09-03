package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/shrimp-music/backend/db"
	"golang.org/x/crypto/bcrypt"
)

type RegisterReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginReq struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
}

func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	userID := uuid.New().String()
	_, err = db.DB.Exec("INSERT INTO users (id, email, password_hash) VALUES (?, ?, ?)", userID, req.Email, string(hashedPassword))
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			http.Error(w, "Email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User registered successfully", "user_id": userID})
}

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	var userID, hash string
	err := db.DB.QueryRow("SELECT id, password_hash FROM users WHERE email = ?", req.Email).Scan(&userID, &hash)
	if err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)); err != nil {
		http.Error(w, "Invalid email or password", http.StatusUnauthorized)
		return
	}

	// Generate Token
	rawToken := uuid.New().String()
	
	// Hash the token for storage
	h := sha256.New()
	h.Write([]byte(rawToken))
	tokenHash := hex.EncodeToString(h.Sum(nil))

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "Unknown Device"
	}

	ipAddress := r.Header.Get("X-Forwarded-For")
	if ipAddress == "" {
		ipAddress = r.RemoteAddr
	}

	tokenID := uuid.New().String()
	_, err = db.DB.Exec("INSERT INTO user_tokens (id, user_id, token_hash, device_name, ip_address) VALUES (?, ?, ?, ?, ?)",
		tokenID, userID, tokenHash, deviceName, ipAddress)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{
		"token": rawToken, // Give raw token to user
	})
}

func DevicesHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)

	rows, err := db.DB.Query("SELECT id, device_name, ip_address, last_used_at FROM user_tokens WHERE user_id = ? AND is_revoked = 0 ORDER BY last_used_at DESC", userID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var devices []map[string]interface{}
	for rows.Next() {
		var id, name, ip, lastUsed string
		if err := rows.Scan(&id, &name, &ip, &lastUsed); err != nil {
			continue
		}
		devices = append(devices, map[string]interface{}{
			"id":           id,
			"device_name":  name,
			"ip_address":   ip,
			"last_used_at": lastUsed,
		})
	}

	json.NewEncoder(w).Encode(devices)
}

func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	tokenHash := r.Context().Value("token_hash").(string)
	_, err := db.DB.Exec("UPDATE user_tokens SET is_revoked = 1 WHERE token_hash = ?", tokenHash)
	if err != nil {
		http.Error(w, "Failed to logout", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

func LogoutAllHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	_, err := db.DB.Exec("UPDATE user_tokens SET is_revoked = 1 WHERE user_id = ?", userID)
	if err != nil {
		http.Error(w, "Failed to logout from all devices", http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out from all devices"})
}


type CheckUsernameReq struct {
	Email string `json:"email"`
}

func CheckUsernameHandler(w http.ResponseWriter, r *http.Request) {
	var req CheckUsernameReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "Username is required", http.StatusBadRequest)
		return
	}

	var exists bool
	err := db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", req.Email).Scan(&exists)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"exists": exists,
	})
}
