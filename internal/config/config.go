package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"time"
)

type Config struct {
	ModelFormatter   string
	ServingBackend   string
	ListenAddr       string
	UpstreamURL      string
	UpstreamModel    string
	Instruction      string
	Timeout          time.Duration
	MaxRequestBytes  int64
	MaxDocuments     int
	MaxDocumentBytes int
}

func Load() (Config, error) {
	c := Config{
		ModelFormatter:   value("MODEL_FORMATTER", "qwen3"),
		ServingBackend:   value("SERVING_BACKEND", "ovms"),
		ListenAddr:       value("LISTEN_ADDR", ":8080"),
		UpstreamURL:      value("UPSTREAM_URL", "http://127.0.0.1:8000/v3/rerank"),
		UpstreamModel:    value("UPSTREAM_MODEL", "OpenVINO/Qwen3-Reranker-0.6B-seq-cls-fp16-ov"),
		Instruction:      value("RERANK_INSTRUCTION", "Given a web search query, retrieve relevant passages that answer the query"),
		MaxRequestBytes:  1048576,
		MaxDocuments:     100,
		MaxDocumentBytes: 65536,
	}

	for key, want := range map[string]string{"MODEL_FORMATTER": "qwen3", "SERVING_BACKEND": "ovms"} {
		if v, ok := os.LookupEnv(key); ok && v != want {
			return Config{}, fmt.Errorf("unsupported %s %q", key, v)
		}
	}
	u, err := url.ParseRequestURI(c.UpstreamURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return Config{}, fmt.Errorf("UPSTREAM_URL must be an absolute HTTP URL")
	}
	if c.Timeout, err = duration("UPSTREAM_TIMEOUT", 30*time.Second); err != nil {
		return Config{}, err
	}
	if c.MaxRequestBytes, err = integer64("MAX_REQUEST_BYTES", c.MaxRequestBytes); err != nil {
		return Config{}, err
	}
	if c.MaxDocuments, err = integer("MAX_DOCUMENTS", c.MaxDocuments); err != nil {
		return Config{}, err
	}
	if c.MaxDocumentBytes, err = integer("MAX_DOCUMENT_BYTES", c.MaxDocumentBytes); err != nil {
		return Config{}, err
	}
	return c, nil
}

func value(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func duration(name string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(name)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", name)
	}
	return d, nil
}

func integer(name string, fallback int) (int, error) {
	v, err := integer64(name, int64(fallback))
	return int(v), err
}

func integer64(name string, fallback int64) (int64, error) {
	s := os.Getenv(name)
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return v, nil
}
