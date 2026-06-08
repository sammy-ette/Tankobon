package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type KavitaClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func NewKavitaClient(httpClient *http.Client, baseURL string, apiKey string) *KavitaClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &KavitaClient{httpClient: httpClient, baseURL: baseURL, apiKey: apiKey}
}

func (c *KavitaClient) do(ctx context.Context, method, path string, query url.Values, body io.Reader, v interface{}) error {
	reqURL, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if v == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

func (c *KavitaClient) ScanLibrary(ctx context.Context, libraryPath string) error {
	var req struct {
		ApiKey               string `json:"apiKey"`
		FolderPath           string `json:"folderPath"`
		AbortOnNoSeriesMatch bool   `json:"abortOnNoSeriesMatch"`
	}

	req.ApiKey = c.apiKey
	req.FolderPath = libraryPath
	req.AbortOnNoSeriesMatch = false

	jsonBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	return c.do(ctx, http.MethodPost, "/api/series/scan-folder", nil, bytes.NewBuffer(jsonBody), nil)
}
