package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"github.com/google/uuid"
	"github.com/shrimp-music/backend/db"
)

type Playlist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
}

type Track struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Artist          string `json:"artist"`
	Thumbnail       string `json:"thumbnail"`
	DurationSeconds int    `json:"duration_seconds"`
}

type PlaylistTrack struct {
	ID            int    `json:"id"`
	PlaylistID    string `json:"playlist_id"`
	Track         Track  `json:"track"`
	PositionOrder int    `json:"position_order"`
	AddedAt       string `json:"added_at"`
}

func GetPlaylistsHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	
	rows, err := db.DB.Query("SELECT id, name, description, created_at FROM playlists WHERE user_id = ? ORDER BY created_at DESC", userID)
	if err != nil {
		http.Error(w, "Failed to get playlists", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	playlists := []Playlist{}
	for rows.Next() {
		var p Playlist
		var desc sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &desc, &p.CreatedAt); err != nil {
			continue
		}
		p.Description = desc.String
		playlists = append(playlists, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(playlists)
}

func CreatePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Playlist name is required", http.StatusBadRequest)
		return
	}

	id := uuid.New().String()
	_, err := db.DB.Exec("INSERT INTO playlists (id, user_id, name, description) VALUES (?, ?, ?, ?)", id, userID, req.Name, req.Description)
	if err != nil {
		http.Error(w, "Failed to create playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"id": id, "message": "Playlist created"})
}

func DeletePlaylistHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	playlistID := r.URL.Query().Get("id")

	if playlistID == "" {
		http.Error(w, "Playlist ID is required", http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec("DELETE FROM playlists WHERE id = ? AND user_id = ?", playlistID, userID)
	if err != nil {
		http.Error(w, "Failed to delete playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Playlist deleted"})
}


func GetPlaylistTracksHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	playlistID := r.URL.Query().Get("id")

	if playlistID == "" {
		http.Error(w, "Playlist ID is required", http.StatusBadRequest)
		return
	}

	// Verify ownership
	var exists bool
	db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id = ? AND user_id = ?)", playlistID, userID).Scan(&exists)
	if !exists {
		http.Error(w, "Playlist not found or unauthorized", http.StatusNotFound)
		return
	}

	rows, err := db.DB.Query(`
		SELECT pt.id, pt.position_order, pt.added_at, 
		       t.id, t.title, t.artist, t.thumbnail, t.duration_seconds
		FROM playlist_tracks pt
		JOIN tracks t ON pt.track_id = t.id
		WHERE pt.playlist_id = ?
		ORDER BY pt.position_order ASC`, playlistID)
		
	if err != nil {
		http.Error(w, "Failed to get tracks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var tracks []PlaylistTrack
	for rows.Next() {
		var pt PlaylistTrack
		var thumb sql.NullString
		if err := rows.Scan(&pt.ID, &pt.PositionOrder, &pt.AddedAt, 
		                    &pt.Track.ID, &pt.Track.Title, &pt.Track.Artist, &thumb, &pt.Track.DurationSeconds); err != nil {
			continue
		}
		pt.Track.Thumbnail = thumb.String
		pt.PlaylistID = playlistID
		tracks = append(tracks, pt)
	}
	
	if tracks == nil {
		tracks = []PlaylistTrack{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tracks)
}

func AddTrackToPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	playlistID := r.URL.Query().Get("id")
	
	var req Track
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid input", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Title == "" {
		http.Error(w, "Track ID and title are required", http.StatusBadRequest)
		return
	}

	// Verify ownership
	var exists bool
	db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id = ? AND user_id = ?)", playlistID, userID).Scan(&exists)
	if !exists {
		http.Error(w, "Playlist not found or unauthorized", http.StatusNotFound)
		return
	}

	// Upsert track
	_, err := db.DB.Exec(`
		INSERT INTO tracks (id, title, artist, thumbnail, duration_seconds) 
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET play_count=play_count+1`, 
		req.ID, req.Title, req.Artist, req.Thumbnail, req.DurationSeconds)
	if err != nil {
		http.Error(w, "Failed to save track", http.StatusInternalServerError)
		return
	}

	// Get max position
	var maxPos int
	db.DB.QueryRow("SELECT COALESCE(MAX(position_order), 0) FROM playlist_tracks WHERE playlist_id = ?", playlistID).Scan(&maxPos)

	// Insert playlist_track
	_, err = db.DB.Exec("INSERT INTO playlist_tracks (playlist_id, track_id, position_order) VALUES (?, ?, ?)", 
		playlistID, req.ID, maxPos+1)
	if err != nil {
		http.Error(w, "Failed to add track to playlist", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "Track added"})
}


func RemoveTrackFromPlaylistHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(string)
	playlistID := r.URL.Query().Get("id")
	trackID := r.URL.Query().Get("track_id")

	if playlistID == "" || trackID == "" {
		http.Error(w, "Playlist ID and Track ID are required", http.StatusBadRequest)
		return
	}

	// Verify ownership
	var exists bool
	db.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM playlists WHERE id = ? AND user_id = ?)", playlistID, userID).Scan(&exists)
	if !exists {
		http.Error(w, "Playlist not found or unauthorized", http.StatusNotFound)
		return
	}

	_, err := db.DB.Exec("DELETE FROM playlist_tracks WHERE playlist_id = ? AND track_id = ?", playlistID, trackID)
	if err != nil {
		http.Error(w, "Failed to remove track", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Track removed"})
}


