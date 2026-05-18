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
	MagnetURI   string `json:"magnetUrl"`
	PublishDate string `json:"publishDate"`
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

func (c *ProwlarrClient) GetMagnetURI(downloadURL string) (string, error) {
	if downloadURL == "" {
		return "", fmt.Errorf("download URL is empty")
	}

	client := &http.Client{
		Timeout: c.timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("prowlarr download request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		redirectURL := resp.Header.Get("Location")
		if redirectURL == "" {
			return "", fmt.Errorf("prowlarr returned redirect but no Location header")
		}
		return redirectURL, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("prowlarr returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	magnetURI := string(body)
	if magnetURI == "" {
		return "", fmt.Errorf("prowlarr returned empty magnet URI")
	}

	return magnetURI, nil
}
