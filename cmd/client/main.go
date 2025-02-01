package main

import (
	"context"
	"database/sql"
	"flag"
	"os"
	"os/signal"
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
	configPath := flag.String("config", "config.json", "Path to configuration file")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		panic(err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Открытие подключения к БД для клиентского репозитория.
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

	// Инициализация бизнес-логики клиента.
	clientService := service.NewClientService(repo, logger)

	// Транспортный слой клиента (WebSocket-соединение).
	ct := transportClient.NewClientTransport(cfg.ClientServerURL, clientService, logger)

	ctx, cancel := context.WithCancel(context.Background())
	go ct.Listen(ctx)

	// Graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	logger.Info("Shutting down client...")
	cancel()
	time.Sleep(2 * time.Second)
	logger.Info("Client stopped")
}
