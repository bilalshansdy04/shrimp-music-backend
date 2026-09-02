package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shrimp-music/backend/ytdlp"
)

func SearchSongs(ctx context.Context, query string) ([]ytdlp.SearchResult, error) {
	url := "https://music.youtube.com/youtubei/v1/search"
	
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB_REMIX",
				"clientVersion": "1.20230524.01.00",
			},
		},
		"query": query,
		"params": "Eg-KAQwIARAUGAMgAQ==", // YouTube Music 'Songs' filter
	}
	
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	// Optional: add User-Agent to avoid blocks
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	
	var results []ytdlp.SearchResult
	
	// Helper to find musicResponsiveListItemRenderer
	var findItems func(v interface{}) []map[string]interface{}
	findItems = func(v interface{}) []map[string]interface{} {
		var res []map[string]interface{}
		switch val := v.(type) {
		case map[string]interface{}:
			if item, ok := val["musicResponsiveListItemRenderer"]; ok {
				if m, ok := item.(map[string]interface{}); ok {
					res = append(res, m)
				}
			}
			for _, child := range val {
				res = append(res, findItems(child)...)
			}
		case []interface{}:
			for _, child := range val {
				res = append(res, findItems(child)...)
			}
		}
		return res
	}
	
	items := findItems(data)
	
	for _, item := range items {
		var videoId, title, artist, thumbnail string
		isInvalid := false
		
		// 1. Get Video ID
		if overlay, ok := item["overlay"].(map[string]interface{}); ok {
			if renderer, ok := overlay["musicItemThumbnailOverlayRenderer"].(map[string]interface{}); ok {
				if content, ok := renderer["content"].(map[string]interface{}); ok {
					if playBtn, ok := content["musicPlayButtonRenderer"].(map[string]interface{}); ok {
						if endp, ok := playBtn["playNavigationEndpoint"].(map[string]interface{}); ok {
							if watchEndp, ok := endp["watchEndpoint"].(map[string]interface{}); ok {
								if vid, ok := watchEndp["videoId"].(string); ok {
									videoId = vid
								}
							}
						}
					}
				}
			}
		}
		
		if videoId == "" {
			continue // skip if not a playable track
		}
		
		// 2. Get Thumbnail
		if thumbObj, ok := item["thumbnail"].(map[string]interface{}); ok {
			if renderer, ok := thumbObj["musicThumbnailRenderer"].(map[string]interface{}); ok {
				if thumb, ok := renderer["thumbnail"].(map[string]interface{}); ok {
					if thumbList, ok := thumb["thumbnails"].([]interface{}); ok && len(thumbList) > 0 {
						if lastThumb, ok := thumbList[len(thumbList)-1].(map[string]interface{}); ok {
							thumbnail, _ = lastThumb["url"].(string)
						}
					}
				}
			}
		}
		
		// 3. Get Title & Artist
		if flexColumns, ok := item["flexColumns"].([]interface{}); ok {
			// Title is usually in flexColumns[0]
			if len(flexColumns) > 0 {
				if col0, ok := flexColumns[0].(map[string]interface{}); ok {
					if renderer, ok := col0["musicResponsiveListItemFlexColumnRenderer"].(map[string]interface{}); ok {
						if text, ok := renderer["text"].(map[string]interface{}); ok {
							if runs, ok := text["runs"].([]interface{}); ok && len(runs) > 0 {
								if run0, ok := runs[0].(map[string]interface{}); ok {
									title, _ = run0["text"].(string)
								}
							}
						}
					}
				}
			}
			
			// Artist is usually in flexColumns[1]
			if len(flexColumns) > 1 {
				if col1, ok := flexColumns[1].(map[string]interface{}); ok {
					if renderer, ok := col1["musicResponsiveListItemFlexColumnRenderer"].(map[string]interface{}); ok {
						if text, ok := renderer["text"].(map[string]interface{}); ok {
							if runs, ok := text["runs"].([]interface{}); ok {
								var artistParts []string
								for _, run := range runs {
									if r, ok := run.(map[string]interface{}); ok {
										if t, ok := r["text"].(string); ok {
											// Clean up separators
											if t == "Episode" || t == "Podcast" {
												isInvalid = true
											}
											if t != " • " && t != "Song" && !strings.Contains(t, "views") {
												artistParts = append(artistParts, t)
											}
										}
									}
								}
								artist = strings.Join(artistParts, ", ")
							}
						}
					}
				}
			}
		}
		
		if isInvalid {
			fmt.Printf("Filtered out: %s by %s\n", title, artist)
			continue // Skip podcast/episode results
		}
		
		results = append(results, ytdlp.SearchResult{
			ID:        videoId,
			Title:     title,
			Artist:    artist,
			Thumbnail: thumbnail,
			Duration:  0,
		})
		
		if len(results) >= 10 {
			break
		}
	}
	
	if len(results) == 0 {
		return []ytdlp.SearchResult{}, nil
	}
	
	return results, nil
}
