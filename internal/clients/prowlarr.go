package clients

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type ProwlarrClient struct {
	baseURL  string
	apiToken string
	timeout  time.Duration
}

type SearchResult struct {
	Title       string `json:"title"`
	Indexer     string `json:"indexer"`
	Seeders     int    `json:"seeders"`
	Leechers    int    `json:"leechers"`
	Size        int64  `json:"size"`
	PublishDate string `json:"publishDate"`
	DownloadURL string `json:"downloadUrl"`
}

func NewProwlarr(baseURL, apiToken string) *ProwlarrClient {
	return &ProwlarrClient{
		baseURL:  baseURL,
		apiToken: apiToken,
		timeout:  10 * time.Second,
	}
}

func (c *ProwlarrClient) Search(query string) ([]SearchResult, error) {
	if c.baseURL == "" || c.apiToken == "" {
		return nil, fmt.Errorf("prowlarr not configured")
	}

	params := url.Values{}
	params.Add("query", query)
	params.Add("type", "search")
	params.Add("apikey", c.apiToken)
	params.Add("categories", "7000")

	searchURL := fmt.Sprintf("%s/api/v1/search?%s", c.baseURL, params.Encode())

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Get(searchURL)
	if err != nil {
		return nil, fmt.Errorf("prowlarr search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr returned %d: %s", resp.StatusCode, string(body))
	}

	var results []SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return nil, fmt.Errorf("decode prowlarr response: %w", err)
	}

	return results, nil
}

func (c *ProwlarrClient) GetTorrentFile(downloadURL string) ([]byte, error) {
	if downloadURL == "" {
		return nil, fmt.Errorf("download URL is empty")
	}

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return nil, fmt.Errorf("prowlarr download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("prowlarr returned %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read prowlarr response: %w", err)
	}

	return data, nil
}
