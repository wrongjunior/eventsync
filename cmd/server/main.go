package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wrongjunior/eventsync/internal/config"
	"github.com/wrongjunior/eventsync/internal/service"
	transportServer "github.com/wrongjunior/eventsync/internal/transport/server"
	"log/slog"
)

func main() {
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Инициализация бизнес-логики сервера.
	eventService := service.NewEventService(logger)
	eventService.StartEventGenerator()

	router := transportServer.SetupRouter(eventService, logger, cfg.WSPath)
	httpServer := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: router,
	}

	// Запуск HTTP-сервера.
	go func() {
		logger.Info("Starting HTTP server", "addr", cfg.ServerAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Обработка graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}
	eventService.Shutdown()
	logger.Info("Server stopped")
}
