package core

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"nextbox/internal/i18n"
	"strconv"
	"time"
)

// API is a minimal client for the sing-box clash API.
type API struct {
	Addr   string // host:port
	Secret string
	client *http.Client
}

func NewAPI(addr, secret string) *API {
	return &API{Addr: addr, Secret: secret, client: &http.Client{}}
}

func (a *API) request(ctx context.Context, method, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, "http://"+a.Addr+path, nil)
	if err != nil {
		return nil, err
	}
	if a.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+a.Secret)
	}
	return a.client.Do(req)
}

func (a *API) getJSON(ctx context.Context, path string, v any) error {
	resp, err := a.request(ctx, http.MethodGet, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiError(resp)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func apiError(resp *http.Response) error {
	var e struct {
		Message string `json:"message"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if json.Unmarshal(body, &e) == nil && e.Message != "" {
		return errors.New(e.Message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}

func (a *API) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	var v map[string]any
	return a.getJSON(ctx, "/version", &v)
}

// Delay runs a URL test through the outbound with the given tag.
func (a *API) Delay(ctx context.Context, tag, testURL string, timeoutMs int) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs+2000)*time.Millisecond)
	defer cancel()
	q := url.Values{}
	q.Set("url", testURL)
	q.Set("timeout", strconv.Itoa(timeoutMs))
	var v struct {
		Delay int `json:"delay"`
	}
	if err := a.getJSON(ctx, "/proxies/"+url.PathEscape(tag)+"/delay?"+q.Encode(), &v); err != nil {
		return -1, err
	}
	if v.Delay <= 0 {
		return -1, errors.New(i18n.T("超时"))
	}
	return v.Delay, nil
}

type ConnMetadata struct {
	Network         string `json:"network"`
	Type            string `json:"type"`
	SourceIP        string `json:"sourceIP"`
	DestinationIP   string `json:"destinationIP"`
	SourcePort      string `json:"sourcePort"`
	DestinationPort string `json:"destinationPort"`
	Host            string `json:"host"`
	ProcessPath     string `json:"processPath"`
}

type Connection struct {
	ID       string       `json:"id"`
	Metadata ConnMetadata `json:"metadata"`
	Upload   int64        `json:"upload"`
	Download int64        `json:"download"`
	Start    string       `json:"start"`
	Chains   []string     `json:"chains"`
	Rule     string       `json:"rule"`
}

type Connections struct {
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Memory        int64        `json:"memory"`
	Connections   []Connection `json:"connections"`
}

func (a *API) Connections(ctx context.Context) (*Connections, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	v := &Connections{}
	if err := a.getJSON(ctx, "/connections", v); err != nil {
		return nil, err
	}
	if v.Connections == nil {
		v.Connections = []Connection{}
	}
	return v, nil
}

func (a *API) CloseConnection(ctx context.Context, id string) error {
	path := "/connections"
	if id != "" {
		path += "/" + url.PathEscape(id)
	}
	resp, err := a.request(ctx, http.MethodDelete, path)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// StreamTraffic calls fn every second with the current up/down speed (bytes/s)
// until ctx is cancelled or the core goes away.
func (a *API) StreamTraffic(ctx context.Context, fn func(up, down int64)) error {
	resp, err := a.request(ctx, http.MethodGet, "/traffic")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return apiError(resp)
	}
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var t struct {
			Up   int64 `json:"up"`
			Down int64 `json:"down"`
		}
		if json.Unmarshal(sc.Bytes(), &t) == nil {
			fn(t.Up, t.Down)
		}
	}
	return sc.Err()
}
