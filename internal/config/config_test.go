package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("LISTEN_ADDR", "")
	t.Setenv("UPSTREAM_URL", "")
	t.Setenv("UPSTREAM_MODEL", "")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != ":8080" || cfg.UpstreamURL != "http://127.0.0.1:8000/v3/rerank" {
		t.Fatalf("unexpected defaults: %#v", cfg)
	}
	if cfg.Timeout != 30*time.Second || cfg.MaxDocuments != 100 {
		t.Fatalf("unexpected limits: %#v", cfg)
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("UPSTREAM_URL", "not-a-url")
	if _, err := Load(); err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestPluginSelectors(t *testing.T) {
	for _, key := range []string{"MODEL_FORMATTER", "SERVING_BACKEND"} {
		for _, invalid := range []string{"", "unknown"} {
			t.Run(key+"/"+invalid, func(t *testing.T) {
				t.Setenv(key, invalid)
				if _, err := Load(); err == nil {
					t.Fatal("expected invalid plugin selector to fail")
				}
			})
		}
	}
}
