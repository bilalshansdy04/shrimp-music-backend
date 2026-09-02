package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/shrimp-music/backend/ytdlp"
)

type UniversalSearchResult struct {
	Tracks  []ytdlp.SearchResult "json:\"tracks\""
	Artists []ytdlp.SearchResult "json:\"artists\""
	Albums  []ytdlp.SearchResult "json:\"albums\""
}

func SearchUniversal(ctx context.Context, query string) (*UniversalSearchResult, error) {
	url := "https://music.youtube.com/youtubei/v1/search"
	
	type ClientCtx struct {
		ClientName    string "json:\"clientName\""
		ClientVersion string "json:\"clientVersion\""
	}
	type Context struct {
		Client ClientCtx "json:\"client\""
	}
	type SearchPayload struct {
		Context Context "json:\"context\""
		Query   string  "json:\"query\""
	}
	
	payload := SearchPayload{
		Context: Context{
			Client: ClientCtx{
				ClientName:    "WEB_REMIX",
				ClientVersion: "1.20230524.01.00",
			},
		},
		Query: query,
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
	
	result := &UniversalSearchResult{
		Tracks:  make([]ytdlp.SearchResult, 0),
		Artists: make([]ytdlp.SearchResult, 0),
		Albums:  make([]ytdlp.SearchResult, 0),
	}
	
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
		var id, title, subtitle, thumbnail string
		var entityType string 
		
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
		
		if flexColumns, ok := item["flexColumns"].([]interface{}); ok {
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
			if len(flexColumns) > 1 {
				if col1, ok := flexColumns[1].(map[string]interface{}); ok {
					if renderer, ok := col1["musicResponsiveListItemFlexColumnRenderer"].(map[string]interface{}); ok {
						if text, ok := renderer["text"].(map[string]interface{}); ok {
							if runs, ok := text["runs"].([]interface{}); ok {
								var parts []string
								for _, run := range runs {
									if r, ok := run.(map[string]interface{}); ok {
										if t, ok := r["text"].(string); ok {
											if t != " • " {
												parts = append(parts, t)
											}
										}
									}
								}
								subtitle = strings.Join(parts, " • ")
							}
						}
					}
				}
			}
		}
		
		if nav, ok := item["navigationEndpoint"].(map[string]interface{}); ok {
			if browse, ok := nav["browseEndpoint"].(map[string]interface{}); ok {
				if bId, ok := browse["browseId"].(string); ok {
					id = bId
					if strings.HasPrefix(bId, "UC") {
						entityType = "artist"
					} else if strings.HasPrefix(bId, "MPRE") {
						entityType = "album"
					}
				}
			} else if watch, ok := nav["watchEndpoint"].(map[string]interface{}); ok {
				if vId, ok := watch["videoId"].(string); ok {
					id = vId
					entityType = "track"
				}
			}
		}
		
		if id == "" {
			if overlay, ok := item["overlay"].(map[string]interface{}); ok {
				if renderer, ok := overlay["musicItemThumbnailOverlayRenderer"].(map[string]interface{}); ok {
					if content, ok := renderer["content"].(map[string]interface{}); ok {
						if playBtn, ok := content["musicPlayButtonRenderer"].(map[string]interface{}); ok {
							if endp, ok := playBtn["playNavigationEndpoint"].(map[string]interface{}); ok {
								if watchEndp, ok := endp["watchEndpoint"].(map[string]interface{}); ok {
									if vId, ok := watchEndp["videoId"].(string); ok {
										id = vId
										entityType = "track"
									}
								}
							}
						}
					}
				}
			}
		}
		
		if id == "" || entityType == "" {
			continue 
		}

		resItem := ytdlp.SearchResult{
			ID:        id,
			Title:     title,
			Artist:    subtitle, 
			Thumbnail: thumbnail,
			Duration:  0,
		}

		if entityType == "track" {
			if strings.Contains(resItem.Artist, "views") || strings.Contains(resItem.Artist, "Episode") || strings.Contains(resItem.Artist, "Podcast") {
				continue
			}
			resItem.Artist = strings.Replace(resItem.Artist, "Song • ", "", 1)
			resItem.Artist = strings.Replace(resItem.Artist, "Video • ", "", 1)
			exists := false
			for _, t := range result.Tracks {
				if t.ID == resItem.ID {
					exists = true
					break
				}
			}
			if !exists {
				result.Tracks = append(result.Tracks, resItem)
			}
		} else if entityType == "artist" {
			resItem.Artist = strings.Replace(resItem.Artist, "Artist • ", "", 1)
			result.Artists = append(result.Artists, resItem)
		} else if entityType == "album" {
			resItem.Artist = strings.Replace(resItem.Artist, "Single • ", "", 1)
			resItem.Artist = strings.Replace(resItem.Artist, "EP • ", "", 1)
			resItem.Artist = strings.Replace(resItem.Artist, "Album • ", "", 1)
			result.Albums = append(result.Albums, resItem)
		}
	}
	
	return result, nil
}