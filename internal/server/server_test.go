package server

import (
	"os"
	"testing"
	"time"

	"log/slog"
)

func TestServerBroadcast(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	srv := NewServer(logger)
	go srv.Run()

	// Создаём фиктивного клиента с буферизованным каналом.
	client := &Client{
		send: make(chan Event, 1),
	}
	srv.register <- client

	event := Event{
		ID:        "test1",
		Type:      "info",
		Message:   "Test event",
		Timestamp: time.Now(),
	}
	srv.Broadcast(event)

	select {
	case received := <-client.send:
		if received.ID != event.ID {
			t.Errorf("Ожидался ID %s, получен %s", event.ID, received.ID)
		}
	case <-time.After(2 * time.Second):
		t.Error("Таймаут ожидания события")
	}
}
