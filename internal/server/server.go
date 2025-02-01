package server

import (
	"context"
	_ "encoding/json"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// Event представляет событие с дополнительным полем Type.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Client представляет подключённого клиента по WebSocket.
type Client struct {
	conn *websocket.Conn
	send chan Event
}

// Broadcaster определяет интерфейс для рассылки событий.
type Broadcaster interface {
	Broadcast(event Event)
}

// Server реализует сервер с поддержкой подписки клиентов, генерацией событий и graceful shutdown.
type Server struct {
	clients    map[*Client]struct{}
	mu         sync.RWMutex
	broadcast  chan Event
	register   chan *Client
	unregister chan *Client
	logger     *slog.Logger
	ctx        context.Context
	cancel     context.CancelFunc
}

// upgrader выполняет апгрейд HTTP-соединения до WebSocket.
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Разрешаем подключения с любых источников (демо-режим)
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// NewServer создаёт новый экземпляр сервера с заданным логгером.
func NewServer(logger *slog.Logger) *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan Event),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Run запускает обработку каналов и генерацию событий.
func (s *Server) Run() {
	s.logger.Info("Запуск сервера")
	go s.handleChannels()
	go s.eventGenerator(s.ctx)
}

// handleChannels обрабатывает регистрацию/удаление клиентов и рассылку событий.
func (s *Server) handleChannels() {
	for {
		select {
		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = struct{}{}
			s.mu.Unlock()
			s.logger.Info("Клиент зарегистрирован")
		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				s.logger.Info("Клиент отключён")
			}
			s.mu.Unlock()
		case event := <-s.broadcast:
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- event:
				default:
					// Если канал клиента переполнен – отключаем его.
					s.mu.RUnlock()
					s.mu.Lock()
					delete(s.clients, client)
					close(client.send)
					s.mu.Unlock()
					s.mu.RLock()
					s.logger.Warn("Удалён клиент из-за медленной обработки", "client", client)
				}
			}
			s.mu.RUnlock()
			s.logger.Info("Рассылка события", "event", event)
		case <-s.ctx.Done():
			s.logger.Info("Остановка обработки каналов")
			return
		}
	}
}

// Broadcast отправляет событие всем подключённым клиентам.
func (s *Server) Broadcast(event Event) {
	s.broadcast <- event
}

// eventGenerator генерирует события каждые 5 секунд с случайным типом.
func (s *Server) eventGenerator(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	counter := 1
	eventTypes := []string{"info", "warning", "error"}
	for {
		select {
		case <-ticker.C:
			evtType := eventTypes[rand.Intn(len(eventTypes))]
			event := Event{
				ID:        strconv.Itoa(counter),
				Type:      evtType,
				Message:   "Событие номер " + strconv.Itoa(counter),
				Timestamp: time.Now(),
			}
			s.logger.Info("Сгенерировано событие", "event", event)
			s.Broadcast(event)
			counter++
		case <-ctx.Done():
			s.logger.Info("Остановка генератора событий")
			return
		}
	}
}

// HandleWebSocket выполняет апгрейд HTTP-соединения и регистрирует клиента.
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Ошибка апгрейда WebSocket", "error", err)
		return
	}
	client := &Client{
		conn: conn,
		send: make(chan Event, 256),
	}
	s.register <- client

	// Создаём контекст для управления жизненным циклом клиента.
	clientCtx, cancel := context.WithCancel(s.ctx)
	defer cancel()

	go s.writePump(client, clientCtx)
	s.readPump(client)
}

// readPump читает сообщения от клиента (например, пинги) и завершает соединение при ошибке.
func (s *Server) readPump(client *Client) {
	defer func() {
		s.unregister <- client
		client.conn.Close()
	}()
	client.conn.SetReadLimit(1024)
	client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		// Содержимое входящих сообщений не обрабатывается (можно расширить логику)
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("Неожиданное закрытие соединения", "error", err)
			}
			break
		}
	}
}

// writePump отправляет события клиенту и периодически отправляет ping.
func (s *Server) writePump(client *Client, ctx context.Context) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()
	for {
		select {
		case event, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Канал закрыт – завершаем соединение.
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// Используем JSON-энкодирование
			if err := client.conn.WriteJSON(event); err != nil {
				s.logger.Error("Ошибка отправки JSON", "error", err)
				return
			}
		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.logger.Error("Ошибка отправки Ping", "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Завершение writePump для клиента")
			return
		}
	}
}

// Shutdown корректно завершает работу сервера.
func (s *Server) Shutdown() {
	s.cancel()
	s.logger.Info("Сервер завершает работу")
}

// SetupRouter возвращает chi.Router с зарегистрированным WebSocket endpoint.
func (s *Server) SetupRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/ws", s.HandleWebSocket)
	return r
}
