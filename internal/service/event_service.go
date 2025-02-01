package service

import (
	"context"
	"math/rand"
	"strconv"
	"sync"
	"time"

	"github.com/wrongjunior/eventsync/internal/domain"
	"log/slog"
)

// Notifier определяет интерфейс для отправки уведомлений (например, через WebSocket).
type Notifier interface {
	Notify(event domain.Event)
}

// Client представляет абстрактного клиента для рассылки событий.
type Client struct {
	Notifier Notifier
}

// EventService содержит бизнес-логику сервера: регистрацию клиентов, генерацию и рассылку событий.
type EventService struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	logger  *slog.Logger
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewEventService создаёт новый экземпляр сервиса.
func NewEventService(logger *slog.Logger) *EventService {
	ctx, cancel := context.WithCancel(context.Background())
	return &EventService{
		clients: make(map[*Client]struct{}),
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register добавляет клиента для получения уведомлений.
func (s *EventService) Register(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[client] = struct{}{}
	s.logger.Info("Client registered")
}

// Unregister удаляет клиента.
func (s *EventService) Unregister(client *Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, client)
	s.logger.Info("Client unregistered")
}

// Broadcast отправляет событие всем зарегистрированным клиентам.
func (s *EventService) Broadcast(event domain.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for client := range s.clients {
		client.Notifier.Notify(event)
	}
	s.logger.Info("Event broadcast", "event", event)
}

// StartEventGenerator генерирует события каждые 5 секунд.
func (s *EventService) StartEventGenerator() {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		counter := 1
		eventTypes := []string{"info", "warning", "error"}
		for {
			select {
			case <-ticker.C:
				evtType := eventTypes[rand.Intn(len(eventTypes))]
				event := domain.Event{
					ID:        strconv.Itoa(counter),
					Type:      evtType,
					Message:   "Событие номер " + strconv.Itoa(counter),
					Timestamp: time.Now(),
				}
				s.logger.Info("Event generated", "event", event)
				s.Broadcast(event)
				counter++
			case <-s.ctx.Done():
				s.logger.Info("Event generator stopped")
				return
			}
		}
	}()
}

// Shutdown останавливает генератор событий.
func (s *EventService) Shutdown() {
	s.cancel()
	s.logger.Info("EventService shutdown")
}
