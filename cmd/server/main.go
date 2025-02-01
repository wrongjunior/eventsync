package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/wrongjunior/eventsync/internal/server"
)

func main() {
	addr := flag.String("addr", ":8080", "Адрес сервера (например, :8080)")
	flag.Parse()

	// Инициализация slog логгера.
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Создаём экземпляр сервера и запускаем обработку.
	srv := server.NewServer(logger)
	srv.Run()

	// Настраиваем маршруты через chi.
	router := srv.SetupRouter()
	httpServer := &http.Server{
		Addr:    *addr,
		Handler: router,
	}

	// Запуск HTTP-сервера в отдельной горутине.
	go func() {
		logger.Info("Запуск HTTP-сервера", "addr", *addr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Ошибка HTTP-сервера", "error", err)
		}
	}()

	// Обработка graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	logger.Info("Получен сигнал завершения. Остановка сервера...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error("Ошибка при завершении HTTP-сервера", "error", err)
	}
	srv.Shutdown()
	logger.Info("Сервер остановлен")
}
