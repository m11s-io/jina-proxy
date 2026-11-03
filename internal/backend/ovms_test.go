package backend

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/m11s-io/jina-proxy/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOVMSContractAndReadiness(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/health/ready" {
			w.WriteHeader(200)
			return
		}
		if r.URL.Path != "/v3/rerank" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		var body Request
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "model" || body.Query != "query" || len(body.Documents) != 1 || body.Documents[0] != "document" || body.TopN == nil || *body.TopN != 1 || body.ReturnDocuments {
			t.Errorf("unexpected payload: %#v", body)
		}
		w.Write([]byte(`{"results":[{"index":0,"relevance_score":0.75}]}`))
	}))
	defer upstream.Close()
	b, err := New("ovms", config.Config{UpstreamURL: upstream.URL + "/v3/rerank", MaxRequestBytes: 1024}, upstream.Client())
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
	n := 1
	results, err := b.Rerank(context.Background(), Request{Model: "model", Query: "query", Documents: []string{"document"}, TopN: &n})
	if err != nil || len(results) != 1 || results[0].Index != 0 || results[0].RelevanceScore != 0.75 {
		t.Fatalf("results=%#v err=%v", results, err)
	}
}

func TestOVMSFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		limit      int64
		want       error
	}{
		{"status", "", 503, 1024, ErrStatus},
		{"malformed", "not json", 200, 1024, ErrResponse},
		{"bounded", `{"results":[{"index":0,"relevance_score":1}]}`, 200, 8, ErrResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); w.Write([]byte(tc.body)) }))
			defer upstream.Close()
			b, _ := New("ovms", config.Config{UpstreamURL: upstream.URL, MaxRequestBytes: tc.limit}, upstream.Client())
			if _, err := b.Rerank(context.Background(), Request{}); !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want %v", err, tc.want)
			}
			if tc.status == 503 && b.Ready(context.Background()) == nil {
				t.Fatal("unhealthy upstream reported ready")
			}
		})
	}
}

func TestOVMSCancellationAndTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
	}))
	defer upstream.Close()
	client := &http.Client{Timeout: 25 * time.Millisecond}
	b, _ := New("ovms", config.Config{UpstreamURL: upstream.URL, MaxRequestBytes: 1024}, client)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.Rerank(ctx, Request{}); err == nil {
		t.Fatal("cancelled inference succeeded")
	}
	if err := b.Ready(ctx); err == nil {
		t.Fatal("cancelled readiness succeeded")
	}
	if _, err := b.Rerank(context.Background(), Request{}); err == nil {
		t.Fatal("inference ignored timeout")
	}
	if err := b.Ready(context.Background()); err == nil {
		t.Fatal("readiness ignored timeout")
	}
}
