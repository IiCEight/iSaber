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

	"github.com/IiCEight/iSaber/backend/internal/auth"
	"github.com/IiCEight/iSaber/backend/internal/database"
	"github.com/IiCEight/iSaber/backend/internal/httpapi"
	"github.com/IiCEight/iSaber/backend/internal/migrations"
	"github.com/IiCEight/iSaber/backend/internal/publishing"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL is required; see the database setup guide")
	}
	db, err := database.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		return err
	}
	defer db.Close()
	config, err := auth.LoadConfig(os.Getenv)
	if err != nil {
		return err
	}
	service, err := auth.NewService(auth.NewStore(db), config.Secret)
	if err != nil {
		return err
	}
	articles, err := publishing.NewService(db, os.Getenv("UPLOAD_DIR"), publishing.WithCommentOrigin(config.Origin))
	if err != nil {
		return err
	}
	defer articles.Close()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if err := config.ValidateListenAddress(addr); err != nil {
		return err
	}
	server := &http.Server{
		Addr: addr,
		Handler: httpapi.NewHandler(func(ctx context.Context) error {
			return migrations.CheckReady(ctx, db)
		}, httpapi.WithAuthentication(auth.NewHTTPHandler(service, config, articles.AdminHandler())), httpapi.WithPublicArticles(articles.PublicHandler(auth.OptionalOwner(service, config, articles.AssetHandler())))),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); articles.Run(ctx) }()
	defer func() { stop(); <-workerDone }()
	errorsCh := make(chan error, 1)
	go func() {
		logger.Info("API listening", "address", addr)
		errorsCh <- server.ListenAndServe()
	}()
	select {
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		logger.Info("API shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
