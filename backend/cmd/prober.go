package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/marcho4/autoservice-dashboard/backend/internal/config"
	"github.com/marcho4/autoservice-dashboard/backend/pkg/logger"
)

func probe(ctx context.Context) error {
	cfg, err := config.LoadProber()
	if err != nil {
		return err
	}
	log := logger.SetupLogging("autoservice-prober")

	client := &http.Client{Timeout: cfg.ProbeTimeout}
	go MakeProbes(ctx, client, cfg)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	})
	srv := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.HTTPPort),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return runServer(ctx, srv, cfg.ShutdownTimeout, log)
}

func MakeProbes(ctx context.Context, client *http.Client, cfg config.Config) {
	// Заводим счётчики заранее с нулём: иначе fail-серия появляется только после первой ошибки,
	// и rate() не видит первый инкремент.
	for service := range cfg.ProbeTargets {
		metrics.GetOrCreateCounter(fmt.Sprintf(`prober_check{service=%q,result="ok"}`, service))
		metrics.GetOrCreateCounter(fmt.Sprintf(`prober_check{service=%q,result="fail"}`, service))
	}
	for {
		for service, url := range cfg.ProbeTargets {
			probeOnce(ctx, client, service, url)
		}
		slog.Debug("sleeping until next probe", "interval", cfg.ProbeInterval.String())
		select {
		case <-ctx.Done():
			return
		case <-time.After(cfg.ProbeInterval):
		}
	}
}

func probeOnce(ctx context.Context, client *http.Client, service, url string) {
	start := time.Now()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
	}

	if ctx.Err() != nil {
		return
	}

	switch {
	case err != nil:
		metrics.GetOrCreateCounter(fmt.Sprintf(`prober_check{service=%q,result="fail"}`, service)).Inc()
		slog.Error("probe failed", "service", service, "url", url, "error", err)
	case resp.StatusCode != http.StatusOK:
		metrics.GetOrCreateCounter(fmt.Sprintf(`prober_check{service=%q,result="fail"}`, service)).Inc()
		slog.Error("probe failed", "service", service, "url", url, "status", resp.StatusCode)
	default:
		metrics.GetOrCreateCounter(fmt.Sprintf(`prober_check{service=%q,result="ok"}`, service)).Inc()
		metrics.GetOrCreateHistogram(fmt.Sprintf(`prober_check_duration_ms{service=%q}`, service)).Update(float64(time.Since(start).Milliseconds()))
	}
}
