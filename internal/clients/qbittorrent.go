package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
)

type QBClient struct {
	baseURL  string
	username string
	password string
	timeout  time.Duration
	cookie   string
}

type TorrentInfo struct {
	Hash        string  `json:"hash"`
	Name        string  `json:"name"`
	State       string  `json:"state"`
	Progress    float64 `json:"progress"`
	Downloaded  int64   `json:"downloaded"`
	Total       int64   `json:"total_size"`
	Category    string  `json:"category"`
	SavePath    string  `json:"save_path"`
	ContentPath string  `json:"content_path"`
}

type TorrentFile struct {
	Name string `json:"name"`
}

func NewQBClient(baseURL, username, password string) *QBClient {
	return &QBClient{
		baseURL:  baseURL,
		username: username,
		password: password,
		timeout:  10 * time.Second,
	}
}

func (c *QBClient) login() error {
	if c.baseURL == "" || c.username == "" || c.password == "" {
		return fmt.Errorf("qbittorrent not configured")
	}

	client := &http.Client{Timeout: c.timeout}
	data := url.Values{}
	data.Set("username", c.username)
	data.Set("password", c.password)

	resp, err := client.PostForm(fmt.Sprintf("%s/api/v2/auth/login", c.baseURL), data)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed %d: %s", resp.StatusCode, string(body))
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "SID" {
			c.cookie = cookie.Value
			return nil
		}
	}

	return fmt.Errorf("no session cookie returned")
}

var errReauthed = fmt.Errorf("reauthed")

func (c *QBClient) do(req *http.Request) (*http.Response, error) {
	if c.cookie == "" {
		if err := c.login(); err != nil {
			return nil, err
		}
	}

	req.AddCookie(&http.Cookie{Name: "SID", Value: c.cookie})

	client := &http.Client{Timeout: c.timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusForbidden {
		resp.Body.Close()
		c.cookie = ""
		if err := c.login(); err != nil {
			return nil, fmt.Errorf("re-auth failed: %w", err)
		}
		return nil, errReauthed
	}

	return resp, nil
}

func (c *QBClient) AddTorrentFile(torrentData []byte, category string) (string, error) {
	mi, err := metainfo.Load(bytes.NewReader(torrentData))
	if err != nil {
		return "", fmt.Errorf("parse torrent: %w", err)
	}
	hash := mi.HashInfoBytes().HexString()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("torrents", "file.torrent")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(torrentData); err != nil {
		return "", fmt.Errorf("write torrent data: %w", err)
	}
	if category != "" {
		w.WriteField("category", category)
	}
	w.Close()

	if err := c.postMultipart("/api/v2/torrents/add", body.Bytes(), w.FormDataContentType()); err != nil {
		return "", fmt.Errorf("add torrent file: %w", err)
	}

	return hash, nil
}

func (c *QBClient) GetTorrents(category string) ([]TorrentInfo, error) {
	reqURL := fmt.Sprintf("%s/api/v2/torrents/info?category=%s", c.baseURL, url.QueryEscape(category))
	var torrents []TorrentInfo
	if err := c.getJSON(reqURL, &torrents); err != nil {
		return nil, err
	}
	for i := range torrents {
		torrents[i].Progress *= 100
	}
	return torrents, nil
}

func (c *QBClient) StartTorrent(hash string) error {
	data := url.Values{}
	data.Set("hashes", hash)
	if err := c.postForm("/api/v2/torrents/start", data); err != nil {
		return fmt.Errorf("start torrent: %w", err)
	}
	return nil
}

func (c *QBClient) GetTorrent(hash string) (*TorrentInfo, error) {
	reqURL := fmt.Sprintf("%s/api/v2/torrents/info?hashes=%s", c.baseURL, hash)

	var torrents []TorrentInfo
	if err := c.getJSON(reqURL, &torrents); err != nil {
		return nil, err
	}

	for _, t := range torrents {
		if strings.EqualFold(t.Hash, hash) {
			t.Progress *= 100
			return &t, nil
		}
	}

	return nil, fmt.Errorf("torrent not found: %s", hash)
}

func (c *QBClient) GetFiles(hash string) ([]TorrentFile, error) {
	reqURL := fmt.Sprintf("%s/api/v2/torrents/files?hash=%s", c.baseURL, hash)
	var files []TorrentFile
	if err := c.getJSON(reqURL, &files); err != nil {
		return nil, err
	}
	return files, nil
}

func (c *QBClient) RemoveTorrent(hash string, deleteFiles bool) error {
	data := url.Values{}
	data.Set("hashes", hash)
	data.Set("deleteFiles", fmt.Sprintf("%v", deleteFiles))
	if err := c.postForm("/api/v2/torrents/delete", data); err != nil {
		return fmt.Errorf("remove torrent: %w", err)
	}
	return nil
}

func (c *QBClient) CreateCategory(name, path string) error {
	data := url.Values{}
	data.Set("name", name)
	data.Set("savePath", path)
	if err := c.postForm("/api/v2/torrents/createCategory", data); err != nil {
		return fmt.Errorf("create category: %w", err)
	}
	return nil
}

func (c *QBClient) EditCategory(name, path string) error {
	data := url.Values{}
	data.Set("name", name)
	data.Set("savePath", path)
	if err := c.postForm("/api/v2/torrents/editCategory", data); err != nil {
		return fmt.Errorf("edit category: %w", err)
	}
	return nil
}

func (c *QBClient) postForm(path string, data url.Values) error {
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequest("POST", c.baseURL+path, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		return req, nil
	}

	req, err := makeReq()
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err == errReauthed {
		req, err = makeReq()
		if err != nil {
			return err
		}
		req.AddCookie(&http.Cookie{Name: "SID", Value: c.cookie})
		client := &http.Client{Timeout: c.timeout}
		resp, err = client.Do(req)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *QBClient) postMultipart(path string, data []byte, contentType string) error {
	makeReq := func() (*http.Request, error) {
		req, err := http.NewRequest("POST", c.baseURL+path, bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", contentType)
		return req, nil
	}

	req, err := makeReq()
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err == errReauthed {
		req, err = makeReq()
		if err != nil {
			return err
		}
		req.AddCookie(&http.Cookie{Name: "SID", Value: c.cookie})
		client := &http.Client{Timeout: c.timeout}
		resp, err = client.Do(req)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *QBClient) getJSON(reqURL string, out any) error {
	makeReq := func() (*http.Request, error) {
		return http.NewRequest("GET", reqURL, nil)
	}

	req, err := makeReq()
	if err != nil {
		return err
	}

	resp, err := c.do(req)
	if err == errReauthed {
		req, err = makeReq()
		if err != nil {
			return err
		}
		req.AddCookie(&http.Cookie{Name: "SID", Value: c.cookie})
		client := &http.Client{Timeout: c.timeout}
		resp, err = client.Do(req)
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed %d: %s", resp.StatusCode, string(body))
	}

	return json.NewDecoder(resp.Body).Decode(out)
}

