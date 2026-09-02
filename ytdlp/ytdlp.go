package ytdlp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type SearchResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Duration  int    `json:"duration"`
	Thumbnail string `json:"thumbnail"`
}

func Search(ctx context.Context, query string) ([]SearchResult, error) {
	// Use ytsearch5: to get top 5 results
	searchQuery := fmt.Sprintf("ytsearch5:%s", query)
	cmd := exec.CommandContext(ctx, "./yt-dlp.exe",
		"--dump-json",
		"--no-warnings",
		"--default-search", "ytsearch",
		searchQuery,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp search error: %v", err)
	}

	var results []SearchResult
	// yt-dlp returns multiple JSON objects separated by newlines
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}

		id, _ := raw["id"].(string)
		title, _ := raw["title"].(string)
		artist, _ := raw["channel"].(string) // YouTube usually puts channel name here
		durationFloat, _ := raw["duration"].(float64)
		thumbnail, _ := raw["thumbnail"].(string)

		results = append(results, SearchResult{
			ID:        id,
			Title:     title,
			Artist:    artist,
			Duration:  int(durationFloat),
			Thumbnail: thumbnail,
		})
	}

	return results, nil
}

func Resolve(ctx context.Context, videoID string, format string) (string, error) {
	url := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	
	// Default to Best Audio. If format=="video", use Best Video+Audio ("b")
	ytFormat := "ba[ext=m4a]/ba"
	if format == "video" {
		ytFormat = "b"
	}

	cmd := exec.CommandContext(ctx, "./yt-dlp.exe",
		"--no-warnings",
		"--no-call-home",
		"--no-check-certificates",
		"--prefer-free-formats",
		"--youtube-skip-dash-manifest",
		"--skip-download",
		"-g",
		"-f", ytFormat,
		url,
	)

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("yt-dlp resolve error: %v", err)
	}

	return strings.TrimSpace(string(out)), nil
}

// AutoUpdater runs in the background to keep yt-dlp updated
func AutoUpdater(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			UpdateYtDlp()
		case <-ctx.Done():
			return
		}
	}
}

func UpdateYtDlp() error {
	cmd := exec.Command("./yt-dlp.exe", "-U")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to update yt-dlp: %v", err)
	}
	return nil
}
