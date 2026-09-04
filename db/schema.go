package db

import (
	"log"
)

func InitSchema() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS user_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			device_name TEXT NOT NULL,
			ip_address TEXT,
			is_revoked INTEGER DEFAULT 0,
			last_used_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_user_tokens_hash ON user_tokens(token_hash);`,
		`CREATE TABLE IF NOT EXISTS artists (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			avatar_url TEXT,
			banner_url TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS albums (
			id TEXT PRIMARY KEY,
			artist_id TEXT NOT NULL,
			title TEXT NOT NULL,
			cover_url TEXT,
			release_year INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (artist_id) REFERENCES artists (id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS tracks (
			id TEXT PRIMARY KEY,
			artist_id TEXT NOT NULL,
			album_id TEXT,
			title TEXT NOT NULL,
			duration_seconds INTEGER NOT NULL,
			play_count INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (artist_id) REFERENCES artists (id) ON DELETE CASCADE,
			FOREIGN KEY (album_id) REFERENCES albums (id) ON DELETE SET NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_artist ON tracks(artist_id);`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_album ON tracks(album_id);`,
		`CREATE TABLE IF NOT EXISTS playlists (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS playlist_tracks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			playlist_id TEXT NOT NULL,
			track_id TEXT NOT NULL,
			position_order INTEGER NOT NULL,
			added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (playlist_id) REFERENCES playlists (id) ON DELETE CASCADE,
			FOREIGN KEY (track_id) REFERENCES tracks (id) ON DELETE CASCADE
		);`,
		`CREATE INDEX IF NOT EXISTS idx_playlist_tracks ON playlist_tracks(playlist_id, position_order);`,
	}

	for _, query := range queries {
		_, err := DB.Exec(query)
		if err != nil {
			log.Fatalf("Failed to execute schema query: %v\nQuery: %s", err, query)
		}
	}
	log.Println("Database schemas initialized.")
}


