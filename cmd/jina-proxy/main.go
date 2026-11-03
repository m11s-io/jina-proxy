package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/m11s-io/jina-proxy/internal/backend"
	"github.com/m11s-io/jina-proxy/internal/config"
	"github.com/m11s-io/jina-proxy/internal/formatter"
	"github.com/m11s-io/jina-proxy/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(2)
	}

	f, err := formatter.New(cfg.ModelFormatter)
	if err != nil {
		slog.Error("invalid formatter", "error", err)
		os.Exit(2)
	}
	b, err := backend.New(cfg.ServingBackend, cfg, &http.Client{Timeout: cfg.Timeout})
	if err != nil {
		slog.Error("invalid backend", "error", err)
		os.Exit(2)
	}
	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           server.New(cfg, f, b),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", "error", err)
		}
	}()

	slog.Info("jina proxy listening", "address", cfg.ListenAddr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
