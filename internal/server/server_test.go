package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/m11s-io/jina-proxy/internal/backend"
	"github.com/m11s-io/jina-proxy/internal/config"
	"github.com/m11s-io/jina-proxy/internal/formatter"
)

func TestRerankTranslatesJinaRequestAndRestoresDocuments(t *testing.T) {
	var upstreamBody string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(r.Body)
		upstreamBody = string(body)
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"results":[{"index":1,"relevance_score":0.9}]}`))}, nil
	})

	h := newHandler(config.Config{UpstreamURL: "http://ovms/v3/rerank", UpstreamModel: "ov-model", Instruction: "find", Timeout: time.Second, MaxRequestBytes: 1024, MaxDocuments: 10, MaxDocumentBytes: 100}, &http.Client{Transport: transport})
	req := httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"model":"jina-reranker-v2-base-multilingual","query":"q","documents":["a","b"],"top_n":1,"return_documents":true}`))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(upstreamBody, `"model":"ov-model"`) || !strings.Contains(upstreamBody, `\u003cDocument\u003e: b`) {
		t.Fatalf("unexpected upstream body: %s", upstreamBody)
	}
	if !strings.Contains(rec.Body.String(), `"document":{"text":"b"}`) {
		t.Fatalf("original document not restored: %s", rec.Body.String())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHealthAndValidation(t *testing.T) {
	h := newHandler(config.Config{MaxRequestBytes: 1024, MaxDocuments: 1, MaxDocumentBytes: 10}, &http.Client{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("health status=%d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/rerank", strings.NewReader(`{"query":"","documents":[]}`)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyReflectsOVMSHealth(t *testing.T) {
	for _, tc := range []struct{ upstream, want int }{{http.StatusOK, http.StatusOK}, {http.StatusServiceUnavailable, http.StatusServiceUnavailable}} {
		client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v2/health/ready" {
				t.Fatalf("path=%s", r.URL.Path)
			}
			return &http.Response{StatusCode: tc.upstream, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header)}, nil
		})}
		h := newHandler(config.Config{UpstreamURL: "http://ovms/v3/rerank"}, client)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if rec.Code != tc.want {
			t.Fatalf("upstream=%d status=%d", tc.upstream, rec.Code)
		}
	}
}

func newHandler(cfg config.Config, client *http.Client) http.Handler {
	b, _ := backend.New("ovms", cfg, client)
	f, _ := formatter.New("qwen3")
	return New(cfg, f, b)
}
