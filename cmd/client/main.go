package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"log/slog"

	"github.com/wrongjunior/eventsync/internal/client"
)

func main() {
	serverAddr := flag.String("server", "ws://localhost:8080/ws", "Адрес сервера (WebSocket URL)")
	dbPath := flag.String("db", "client.db", "Путь к SQLite базе данных")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cl, err := client.NewClient(*serverAddr, *dbPath, logger)
	if err != nil {
		logger.Error("Ошибка создания клиента", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go cl.Listen(ctx)

	// Обработка сигналов для graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Завершение работы клиента...")
	cancel()
	time.Sleep(2 * time.Second) // Немного ждём для корректного завершения
	logger.Info("Клиент остановлен")
}
