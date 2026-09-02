package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	b, _ := os.ReadFile("innertube.json")
	var data map[string]interface{}
	json.Unmarshal(b, &data)

	// Since traversing map[string]interface{} is deeply nested and fragile,
	// let's do a simple recursive search for "musicResponsiveListItemRenderer"
	var findItems func(v interface{}) []map[string]interface{}
	findItems = func(v interface{}) []map[string]interface{} {
		var results []map[string]interface{}
		switch val := v.(type) {
		case map[string]interface{}:
			if item, ok := val["musicResponsiveListItemRenderer"]; ok {
				if m, ok := item.(map[string]interface{}); ok {
					results = append(results, m)
				}
			}
			for _, child := range val {
				results = append(results, findItems(child)...)
			}
		case []interface{}:
			for _, child := range val {
				results = append(results, findItems(child)...)
			}
		}
		return results
	}

	items := findItems(data)
	for i, item := range items {
		if i >= 5 {
			break
		}
		// Extract VideoID
		var videoId string
		if overlay, ok := item["overlay"].(map[string]interface{}); ok {
			if renderer, ok := overlay["musicItemThumbnailOverlayRenderer"].(map[string]interface{}); ok {
				if content, ok := renderer["content"].(map[string]interface{}); ok {
					if playBtn, ok := content["musicPlayButtonRenderer"].(map[string]interface{}); ok {
						if playNavigationEndpoint, ok := playBtn["playNavigationEndpoint"].(map[string]interface{}); ok {
							if watchEndpoint, ok := playNavigationEndpoint["watchEndpoint"].(map[string]interface{}); ok {
								if vid, ok := watchEndpoint["videoId"].(string); ok {
									videoId = vid
								}
							}
						}
					}
				}
			}
		}

		// Extract Title and Artist from flexColumns
		var title, artist string
		if flexColumns, ok := item["flexColumns"].([]interface{}); ok {
			if len(flexColumns) > 0 {
				if col0, ok := flexColumns[0].(map[string]interface{}); ok {
					if renderer, ok := col0["musicResponsiveListItemFlexColumnRenderer"].(map[string]interface{}); ok {
						if text, ok := renderer["text"].(map[string]interface{}); ok {
							if runs, ok := text["runs"].([]interface{}); ok && len(runs) > 0 {
								if run0, ok := runs[0].(map[string]interface{}); ok {
									title = run0["text"].(string)
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
								var artistParts []string
								for _, run := range runs {
									if r, ok := run.(map[string]interface{}); ok {
										if t, ok := r["text"].(string); ok && t != " • " && t != "Song" {
											artistParts = append(artistParts, t)
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
		
		fmt.Printf("ID: %s | Title: %s | Artist: %s\n", videoId, title, artist)
	}
}
