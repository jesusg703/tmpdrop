// tmpdrop — temporary file sharing. Copyright (C) 2026 jesusg703.
// Licensed under the GNU Affero General Public License v3.0 or later.
// See LICENSE, and section 13: a modified version run as a network service
// must offer its source to the people using it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jesusg703/tmpdrop/internal/config"
	"github.com/jesusg703/tmpdrop/internal/server"
	"github.com/jesusg703/tmpdrop/internal/store"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println(version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid configuration", "error", err)
		os.Exit(2)
	}

	logger = newLogger(cfg.LogLevel)

	st, err := store.Open(cfg.StorageDir, store.Limits{
		MaxFileSize:    cfg.MaxFileSize,
		MaxStorage:     cfg.MaxStorage,
		DefaultTTL:     cfg.DefaultTTL,
		QuotaDefault:   cfg.QuotaDefault,
		MaxFilesClient: cfg.MaxFilesClient,
	})
	if err != nil {
		logger.Error("storage init failed", "error", err)
		os.Exit(1)
	}

	sweepCtx, stopSweep := context.WithCancel(context.Background())
	defer stopSweep()
	go sweepLoop(sweepCtx, st, cfg.SweepInterval, logger)

	srv := server.New(st, cfg, logger)
	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("starting tmpdrop", "version", version, "addr", cfg.Addr, "storage", cfg.StorageDir, "convertx", cfg.ConvertX.Enabled())

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown signal received")
		stopSweep()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("graceful shutdown failed", "error", err)
		}
		logger.Info("shutdown complete")
	}
}

func newLogger(level string) *slog.Logger {
	var lvl slog.Level
	_ = lvl.UnmarshalText([]byte(level))
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}

func sweepLoop(ctx context.Context, st *store.Store, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if removed := st.Sweep(time.Now()); removed > 0 {
				log.Info("removed expired files", "count", removed)
			}
			if st.ReclaimBlocked() {
				log.Warn("stored files are present but the manifest lists none; refusing to reclaim them, restore the manifest or move the files out")
			}
		}
	}
}
