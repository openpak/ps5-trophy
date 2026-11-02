// Command ps5-trophy serves the trophy slice of m.np.playstation.com.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/openpak/ps5-common/oauth"
	"openpak/ps5-trophy/internal/api"
	"openpak/ps5-trophy/internal/config"
	"openpak/ps5-trophy/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(2)
	}
	signer, err := oauth.NewSigner([]byte(cfg.TokenKey))
	if err != nil {
		log.Error("token key", "err", err)
		os.Exit(2)
	}
	st := store.NewMemory()
	if cfg.SetsDir != "" {
		n, err := store.LoadDir(st, cfg.SetsDir)
		if err != nil {
			log.Error("load trophy sets", "err", err)
			os.Exit(2)
		}
		log.Info("trophy sets loaded", "files", n)
	}
	a := &api.Server{St: st, Verifier: signer, InternalKey: cfg.InternalKey, Log: log}
	srv := &http.Server{Addr: cfg.Addr, Handler: a.Handler(), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(sctx)
	}()
	log.Info("ps5-trophy listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}
