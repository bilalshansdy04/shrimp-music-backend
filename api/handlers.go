package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/shrimp-music/backend/cache"
	"github.com/shrimp-music/backend/innertube"
	"github.com/shrimp-music/backend/limiter"
	"github.com/shrimp-music/backend/ytdlp"
)

type API struct {
	cache   *cache.Cache
	limiter limiter.Semaphore
}

func NewAPI(c *cache.Cache, l limiter.Semaphore) *API {
	return &API{
		cache:   c,
		limiter: l,
	}
}

func (api *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/search", api.handleSearch)
	mux.HandleFunc("/api/v1/resolve/", api.handleResolve)
	mux.HandleFunc("/api/v1/artist", api.handleArtist)
	mux.HandleFunc("/api/v1/album", api.handleAlbum)

	// Auth Routes
	mux.HandleFunc("/api/auth/register", RegisterHandler)
	mux.HandleFunc("/api/auth/login", LoginHandler)
	mux.HandleFunc("/api/auth/devices", AuthMiddleware(DevicesHandler))
	mux.HandleFunc("/api/auth/logout", AuthMiddleware(LogoutHandler))
	mux.HandleFunc("/api/auth/logout-all", AuthMiddleware(LogoutAllHandler))
}

func (api *API) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "Missing query parameter 'q'", http.StatusBadRequest)
		return
	}

	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "all"
	}

	// Check Cache
	cacheKey := "search:" + query + ":" + searchType
	if val, ok := api.cache.Get(cacheKey); ok {
		api.jsonResponse(w, http.StatusOK, val)
		return
	}

	// Use Context with Timeout and Concurrency Limiter
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := api.limiter.Acquire(ctx); err != nil {
		http.Error(w, "Server too busy", http.StatusServiceUnavailable)
		return
	}
	defer api.limiter.Release()

	// Perform Universal Search
	results, err := innertube.SearchUniversal(ctx, query)
	if err != nil {
		http.Error(w, "Search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter by searchType if necessary
	var finalData interface{}
	if searchType == "track" {
		finalData = results.Tracks
	} else if searchType == "artist" {
		finalData = results.Artists
	} else if searchType == "album" {
		finalData = results.Albums
	} else {
		finalData = results // all
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   finalData,
	}

	// Cache for 1 hour
	api.cache.Set(cacheKey, response, 1*time.Hour)

	api.jsonResponse(w, http.StatusOK, response)
}

func (api *API) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /api/v1/resolve/{videoId}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 5 || parts[4] == "" {
		http.Error(w, "Missing videoId", http.StatusBadRequest)
		return
	}
	videoID := parts[4]
	format := r.URL.Query().Get("format")
	if format == "" {
		format = "audio" // default
	}

	// Check Cache
	cacheKey := "resolve:" + videoID + ":" + format
	if val, ok := api.cache.Get(cacheKey); ok {
		api.jsonResponse(w, http.StatusOK, val)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if err := api.limiter.Acquire(ctx); err != nil {
		http.Error(w, "Server too busy", http.StatusServiceUnavailable)
		return
	}
	defer api.limiter.Release()

	streamURL, err := ytdlp.Resolve(ctx, videoID, format)
	if err != nil {
		// Try to auto-update yt-dlp if extraction fails (simplified self-healing)
		_ = ytdlp.UpdateYtDlp()
		
		http.Error(w, "Resolve failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	expiresAt := time.Now().Add(4 * time.Hour).Unix()
	response := map[string]interface{}{
		"status": "success",
		"data": map[string]interface{}{
			"id":         videoID,
			"stream_url": streamURL,
			"expires_at": expiresAt,
		},
	}

	// Cache for 4 hours
	api.cache.Set(cacheKey, response, 4*time.Hour)

	api.jsonResponse(w, http.StatusOK, response)
}

func (api *API) jsonResponse(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}


func (api *API) handleArtist(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing query parameter 'id'", http.StatusBadRequest)
		return
	}

	// Check Cache
	cacheKey := "artist:" + id
	if val, ok := api.cache.Get(cacheKey); ok {
		api.jsonResponse(w, http.StatusOK, val)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := api.limiter.Acquire(ctx); err != nil {
		http.Error(w, "Server too busy", http.StatusServiceUnavailable)
		return
	}
	defer api.limiter.Release()

	profile, err := innertube.GetArtistProfile(ctx, id)
	if err != nil {
		http.Error(w, "Failed to get artist profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   profile,
	}

	api.cache.Set(cacheKey, response, 1*time.Hour)
	api.jsonResponse(w, http.StatusOK, response)
}

func (api *API) handleAlbum(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing query parameter 'id'", http.StatusBadRequest)
		return
	}

	// Check Cache
	cacheKey := "album:" + id
	if val, ok := api.cache.Get(cacheKey); ok {
		api.jsonResponse(w, http.StatusOK, val)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if err := api.limiter.Acquire(ctx); err != nil {
		http.Error(w, "Server too busy", http.StatusServiceUnavailable)
		return
	}
	defer api.limiter.Release()

	profile, err := innertube.GetAlbumProfile(ctx, id)
	if err != nil {
		http.Error(w, "Failed to get album profile: "+err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status": "success",
		"data":   profile,
	}

	api.cache.Set(cacheKey, response, 1*time.Hour)
	api.jsonResponse(w, http.StatusOK, response)
}
