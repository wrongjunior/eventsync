package main

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/wrongjunior/eventsync/internal/config"
	"github.com/wrongjunior/eventsync/internal/repository"
	"github.com/wrongjunior/eventsync/internal/service"
	transportClient "github.com/wrongjunior/eventsync/internal/transport/client"
	"log/slog"
)

func main() {
	configPath := flag.String("config", "config/client_config.json", "Path to client configuration file")
	flag.Parse()

	cfg, err := config.LoadClientConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Открываем подключение к БД для клиентского репозитория.
	db, err := sql.Open("sqlite3", cfg.DBPath)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	repo := repository.NewSQLiteRepository(db)
	if err := repo.Init(); err != nil {
		logger.Error("Failed to initialize repository", "error", err)
		os.Exit(1)
	}

	// Инициализируем бизнеслогику клиента.
	clientService := service.NewClientService(repo, logger)

	// Создаем контекст, отменяемый сигналами ОС.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Используем WaitGroup для ожидания завершения всех клиентов.
	var wg sync.WaitGroup

	// Запускаем заданное число клиентов.
	numClients := cfg.NumClients
	logger.Info("Starting clients", "num_clients", numClients)
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			logger.Info("Starting client", "client_id", id)
			transport := transportClient.NewClientTransport(cfg.ClientServerURL, clientService, logger)
			transport.Listen(ctx)
			logger.Info("Client stopped", "client_id", id)
		}(i + 1)
	}

	// Ожидаем сигнал завершения.
	<-ctx.Done()
	logger.Info("Shutdown signal received, waiting for clients to stop...")
	// Ждем завершения всех клиентов (с таймаутом для graceful shutdown).
	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()
	select {
	case <-doneCh:
		logger.Info("All clients stopped gracefully")
	case <-time.After(5 * time.Second):
		logger.Info("Timeout waiting for clients shutdown")
	}
}
