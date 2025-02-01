package server

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/wrongjunior/eventsync/internal/domain"
	eservice "github.com/wrongjunior/eventsync/internal/service"
	"log/slog"
)

// WebSocketNotifier оборачивает websocket-соединение для реализации интерфейса Notifier.
type WebSocketNotifier struct {
	Conn   *websocket.Conn
	Logger *slog.Logger
}

// Notify отправляет событие через WebSocket.
func (w *WebSocketNotifier) Notify(event domain.Event) {
	w.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := w.Conn.WriteJSON(event); err != nil {
		w.Logger.Error("Error writing JSON", "error", err)
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем подключения с любых источников (для демонстрации)
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handler реализует HTTP-обработчик для WebSocket.
type Handler struct {
	EventService *eservice.EventService
	Logger       *slog.Logger
}

// NewHandler создаёт новый обработчик.
func NewHandler(es *eservice.EventService, logger *slog.Logger) *Handler {
	return &Handler{
		EventService: es,
		Logger:       logger,
	}
}

// ServeHTTP выполняет апгрейд соединения и регистрирует клиента.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.Logger.Error("WebSocket upgrade error", "error", err)
		return
	}
	notifier := &WebSocketNotifier{Conn: conn, Logger: h.Logger}
	client := &eservice.Client{Notifier: notifier}
	h.EventService.Register(client)

	// Создаём контекст для управления жизненным циклом соединения.
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go h.writePump(conn, ctx)
	h.readPump(conn)
	h.EventService.Unregister(client)
}

// readPump читает входящие сообщения и завершает соединение при ошибке.
func (h *Handler) readPump(conn *websocket.Conn) {
	defer conn.Close()
	conn.SetReadLimit(1024)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			h.Logger.Error("readPump error", "error", err)
			break
		}
	}
}

// writePump отправляет ping-сообщения для поддержания соединения.
func (h *Handler) writePump(conn *websocket.Conn, ctx context.Context) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				h.Logger.Error("Ping error", "error", err)
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// SetupRouter настраивает маршруты через chi и возвращает http.Handler.
func SetupRouter(es *eservice.EventService, logger *slog.Logger, wsPath string) http.Handler {
	r := chi.NewRouter()
	handler := NewHandler(es, logger)
	r.Get(wsPath, handler.ServeHTTP)
	return r
}
