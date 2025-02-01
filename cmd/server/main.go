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
	configPath := flag.String("config", "config/server_config.json", "Path to server configuration file")
	flag.Parse()

	cfg, err := config.LoadServerConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Инициализация бизнеслогики сервера.
	eventService := service.NewEventService(logger)
	eventService.StartEventGenerator()

	// Настройка маршрутов через chi.
	router := transportServer.SetupRouter(eventService, logger, cfg.WSPath)
	httpServer := &http.Server{
		Addr:    cfg.ServerAddr,
		Handler: router,
	}

	// Используем контекст, отменяемый сигналами ОС.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Запускаем HTTP-сервер в отдельной горутине.
	go func() {
		logger.Info("Starting HTTP server", "addr", cfg.ServerAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("HTTP server error", "error", err)
		}
	}()

	// Ожидаем сигнала завершения.
	<-ctx.Done()
	logger.Info("Shutdown signal received")

	// Инициируем graceful shutdown HTTP-сервера.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("HTTP server shutdown error", "error", err)
	}

	// Завершаем работу генератора событий.
	eventService.Shutdown()
	logger.Info("Server stopped gracefully")
}
