package service

import (
	"sync"

	"github.com/wrongjunior/eventsync/internal/domain"
	"github.com/wrongjunior/eventsync/internal/repository"
	"log/slog"
)

// ClientService реализует бизнеслогику клиента: фильтрация дубликатов и сохранение событий.
type ClientService struct {
	repo        repository.EventRepository
	logger      *slog.Logger
	mu          sync.Mutex
	receivedIDs map[string]struct{}
}

// NewClientService создаёт новый экземпляр клиентского сервиса.
func NewClientService(repo repository.EventRepository, logger *slog.Logger) *ClientService {
	return &ClientService{
		repo:        repo,
		logger:      logger,
		receivedIDs: make(map[string]struct{}),
	}
}

// ProcessEvent фильтрует дубли и сохраняет событие.
func (cs *ClientService) ProcessEvent(event domain.Event) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if _, exists := cs.receivedIDs[event.ID]; exists {
		cs.logger.Info("Duplicate event filtered", "id", event.ID)
		return
	}
	cs.receivedIDs[event.ID] = struct{}{}
	cs.logger.Info("Processing event", "event", event)
	if err := cs.repo.Save(event); err != nil {
		cs.logger.Error("Error saving event", "error", err)
	}
}
