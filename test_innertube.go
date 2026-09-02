package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
)

func main() {
	url := "https://music.youtube.com/youtubei/v1/search"
	
	payload := map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"clientName":    "WEB_REMIX",
				"clientVersion": "1.20230524.01.00",
			},
		},
		"query": "ILLIT Magnetic",
		"params": "Eg-KAQwIARAUGAMgAQ==", // Decoded base64
	}
	
	b, _ := json.Marshal(payload)
	
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	
	body, _ := io.ReadAll(resp.Body)
	os.WriteFile("innertube.json", body, 0644)
}
