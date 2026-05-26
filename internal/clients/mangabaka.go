package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const mangabakaBaseURL = "https://api.mangabaka.dev/v1"

type MangabakaSeries struct {
	ID           int      `json:"id"`
	Title        string   `json:"title"`
	AltTitles    []string `json:"altTitles"`
	CoverURL     string   `json:"coverUrl"`
	Status       string   `json:"status"`
	Year         int      `json:"year"`
	Overview     string   `json:"overview"`
	TotalVolumes int      `json:"totalVolumes"`
}

type apiSeries struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	NativeTitle   string `json:"native_title"`
	RomaizedTitle string `json:"romaized_title"`
	State         string `json:"state"`
	Cover         struct {
		X350 struct {
			X1 *string `json:"x1"`
			X2 *string `json:"x2"`
		} `json:"x350"`
		X250 struct {
			X1 *string `json:"x1"`
		} `json:"x250"`
		X150 struct {
			X1 *string `json:"x1"`
		} `json:"x150"`
	} `json:"cover"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	FinalVolume string  `json:"final_volume"`
	Rating      float64 `json:"rating"`
	Published   struct {
		StartDate string `json:"start_date"`
	} `json:"published"`
	Titles []struct {
		Title     string `json:"title"`
		IsPrimary bool   `json:"is_primary"`
		Language  string `json:"language"`
	} `json:"titles"`
}

type MangabakaClient struct {
	httpClient *http.Client
	baseURL    string
}

func NewMangabaka(httpClient *http.Client) *MangabakaClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &MangabakaClient{httpClient: httpClient, baseURL: mangabakaBaseURL}
}

func (s *apiSeries) normalize() MangabakaSeries {
	coverURL := ""
	switch {
	case s.Cover.X350.X1 != nil && *s.Cover.X350.X1 != "":
		coverURL = *s.Cover.X350.X1
	case s.Cover.X350.X2 != nil && *s.Cover.X350.X2 != "":
		coverURL = *s.Cover.X350.X2
	case s.Cover.X250.X1 != nil && *s.Cover.X250.X1 != "":
		coverURL = *s.Cover.X250.X1
	case s.Cover.X150.X1 != nil && *s.Cover.X150.X1 != "":
		coverURL = *s.Cover.X150.X1
	}

	year := 0
	if len(s.Published.StartDate) >= 4 {
		year, _ = strconv.Atoi(s.Published.StartDate[:4])
	}

	totalVolumes := 0
	if v := strings.TrimSpace(s.FinalVolume); v != "" {
		totalVolumes, _ = strconv.Atoi(v)
	}

	alternateTitles := []string{
		s.RomaizedTitle,
	}

	return MangabakaSeries{
		ID:           s.ID,
		Title:        s.Title,
		AltTitles:    alternateTitles,
		CoverURL:     coverURL,
		Status:       s.Status,
		Year:         year,
		Overview:     s.Description,
		TotalVolumes: totalVolumes,
	}
}

func (c *MangabakaClient) do(ctx context.Context, method, path string, query url.Values, v interface{}) error {
	reqURL, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	reqURL.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	return nil
}

func (c *MangabakaClient) Search(ctx context.Context, query string) ([]MangabakaSeries, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("limit", "20")
	params.Set("sort_by", "relevance_desc")

	var out struct {
		Data []apiSeries `json:"data"`
	}

	if err := c.do(ctx, http.MethodGet, "/series/search", params, &out); err != nil {
		return nil, fmt.Errorf("mangabaka search: %w", err)
	}

	results := make([]MangabakaSeries, len(out.Data))
	for i, s := range out.Data {
		results[i] = s.normalize()
	}
	return results, nil
}

func (c *MangabakaClient) GetSeries(ctx context.Context, id int) (*MangabakaSeries, error) {
	var out struct {
		Data apiSeries `json:"data"`
	}

	path := fmt.Sprintf("/series/%d", id)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, fmt.Errorf("mangabaka get series: %w", err)
	}

	s := out.Data.normalize()
	return &s, nil
}

func (c *MangabakaClient) GetMangaUpdatesSeriesID(ctx context.Context, mangaUpdatesID string) (uint, error) {
	var out struct {
		Data struct {
			SourceResponse struct {
				SeriesID uint `json:"series_id"`
			} `json:"source_response"`
		} `json:"data"`
	}

	params := url.Values{}
	params.Set("with_series", "false")
	params.Set("with_source_response", "true")

	path := fmt.Sprintf("/source/manga-updates/%s", mangaUpdatesID)
	if err := c.do(ctx, http.MethodGet, path, params, &out); err != nil {
		return 0, fmt.Errorf("mangabaka get series: %w", err)
	}

	if out.Data.SourceResponse.SeriesID == 0 {
		return 0, fmt.Errorf("manga updates series id not found for mangaUpdatesID: %s", mangaUpdatesID)
	}

	return out.Data.SourceResponse.SeriesID, nil
}
