package backend

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/m11s-io/jina-proxy/internal/config"
	"io"
	"net/http"
	"net/url"
)

type upstreamRequest struct {
	Model           string   `json:"model"`
	Query           string   `json:"query"`
	Documents       []string `json:"documents"`
	TopN            *int     `json:"top_n,omitempty"`
	ReturnDocuments bool     `json:"return_documents"`
}
type upstreamResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}
type upstreamResponse struct {
	Results []upstreamResult `json:"results"`
}

type Request = upstreamRequest
type Result = upstreamResult
type Backend interface {
	Rerank(context.Context, Request) ([]Result, error)
	Ready(context.Context) error
}
type OVMS struct {
	cfg    config.Config
	client *http.Client
}

func New(name string, cfg config.Config, client *http.Client) (Backend, error) {
	if name != "ovms" {
		return nil, fmt.Errorf("unknown serving backend %q", name)
	}
	return &OVMS{cfg, client}, nil
}
func (h *OVMS) Ready(ctx context.Context) error {
	u, err := url.Parse(h.cfg.UpstreamURL)
	if err != nil || u.Host == "" {
		return fmt.Errorf("upstream unavailable")
	}
	u.Path = "/v2/health/ready"
	u.RawQuery = ""
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return err
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream unavailable")
	}
	return nil
}
func (h *OVMS) Rerank(ctx context.Context, out Request) ([]Result, error) {
	payload, _ := json.Marshal(out)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.cfg.UpstreamURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("upstream unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("upstream error")
	}
	var upstream upstreamResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, h.cfg.MaxRequestBytes)).Decode(&upstream); err != nil {
		return nil, fmt.Errorf("invalid upstream response")
	}

	return upstream.Results, nil
}
