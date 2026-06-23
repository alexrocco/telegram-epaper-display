// Command server runs the telegram-epaper-display backend: it ingests a Telegram
// channel via the Bot API and serves a rendered e-paper frame over HTTP.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexrocco/telegram-epaper-display/backend/internal/api"
	"github.com/alexrocco/telegram-epaper-display/backend/internal/config"
	"github.com/alexrocco/telegram-epaper-display/backend/internal/store"
	"github.com/alexrocco/telegram-epaper-display/backend/internal/telegram"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.New(cfg.StorePath, cfg.MessagesToShow)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	ingester := telegram.New(cfg.BotToken, cfg.ChannelID, st)
	frame := api.NewFrame(st, ingester.Title, cfg.ChannelID, cfg.MessagesToShow)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go ingester.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           frame.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("http: listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http: %v", err)
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http: shutdown: %v", err)
	}
	_ = os.Stdout.Sync()
}
