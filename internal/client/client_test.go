package client

import (
	"testing"
	"time"

	"log/slog"

	_ "github.com/mattn/go-sqlite3"
)

func TestClientDuplicateFiltering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	// Используем in-memory SQLite для теста.
	clRepo, err := NewSQLiteRepository(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := clRepo.Init(); err != nil {
		t.Fatal(err)
	}
	cl := &Client{
		repo:        clRepo,
		logger:      logger,
		receivedIDs: make(map[string]struct{}),
	}

	event := Event{
		ID:        "dup1",
		Type:      "info",
		Message:   "Duplicate Event",
		Timestamp: time.Now(),
	}

	// Обрабатываем событие дважды.
	cl.processEvent(event)
	cl.processEvent(event)

	var count int
	err = clRepo.db.QueryRow("SELECT COUNT(*) FROM events WHERE id = ?", event.ID).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("Ожидалось, что событие сохранится 1 раз, получено %d", count)
	}
}
