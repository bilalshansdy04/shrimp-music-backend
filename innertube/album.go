package innertube

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/shrimp-music/backend/ytdlp"
)

type AlbumProfile struct {
	ID        string               `json:"id"`
	Title     string               `json:"title"`
	Subtitle  string               `json:"subtitle"`
	Thumbnail string               `json:"thumbnail"`
	Tracks    []ytdlp.SearchResult `json:"tracks"`
}

func GetAlbumProfile(ctx context.Context, browseId string) (*AlbumProfile, error) {
	url := "https://music.youtube.com/youtubei/v1/browse"
	
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB_REMIX",
				"clientVersion": "1.20230524.01.00",
			},
		},
		"browseId": browseId,
	}
	
	b, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	
	profile := &AlbumProfile{
		ID:     browseId,
		Tracks: make([]ytdlp.SearchResult, 0),
	}
	
	if contentsObj, ok := data["contents"].(map[string]interface{}); ok {
		if twoCol, ok := contentsObj["twoColumnBrowseResultsRenderer"].(map[string]interface{}); ok {
			
			if tabs, ok := twoCol["tabs"].([]interface{}); ok && len(tabs) > 0 {
				if tab0, ok := tabs[0].(map[string]interface{}); ok {
					if tabRenderer, ok := tab0["tabRenderer"].(map[string]interface{}); ok {
						if content, ok := tabRenderer["content"].(map[string]interface{}); ok {
							if sectionList, ok := content["sectionListRenderer"].(map[string]interface{}); ok {
								if sections, ok := sectionList["contents"].([]interface{}); ok && len(sections) > 0 {
									if section, ok := sections[0].(map[string]interface{}); ok {
										if header, ok := section["musicResponsiveHeaderRenderer"].(map[string]interface{}); ok {
											profile.Title = extractRunsText(header["title"])
											profile.Subtitle = extractRunsText(header["subtitle"])
											
											if thumbObj, ok := header["thumbnail"].(map[string]interface{}); ok {
												if renderer, ok := thumbObj["musicThumbnailRenderer"].(map[string]interface{}); ok {
													if thumb, ok := renderer["thumbnail"].(map[string]interface{}); ok {
														if thumbList, ok := thumb["thumbnails"].([]interface{}); ok && len(thumbList) > 0 {
															if lastThumb, ok := thumbList[len(thumbList)-1].(map[string]interface{}); ok {
																profile.Thumbnail, _ = lastThumb["url"].(string)
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
			
			if secContents, ok := twoCol["secondaryContents"].(map[string]interface{}); ok {
				if sectionList, ok := secContents["sectionListRenderer"].(map[string]interface{}); ok {
					if sections, ok := sectionList["contents"].([]interface{}); ok && len(sections) > 0 {
						if section, ok := sections[0].(map[string]interface{}); ok {
							if shelf, ok := section["musicShelfRenderer"].(map[string]interface{}); ok {
								if items, ok := shelf["contents"].([]interface{}); ok {
									for _, item := range items {
										if iObj, ok := item.(map[string]interface{}); ok {
											if listItem, ok := iObj["musicResponsiveListItemRenderer"].(map[string]interface{}); ok {
												track := parseListItem(listItem, "track")
												if track != nil {
													if track.Thumbnail == "" {
														track.Thumbnail = profile.Thumbnail
													}
													profile.Tracks = append(profile.Tracks, *track)
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	
	return profile, nil
}
