package client

import (
	"context"
	"encoding/json"
	"net/url"
	_ "os"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wrongjunior/eventsync/internal/domain"
	"github.com/wrongjunior/eventsync/internal/repository"
	"log/slog"
)

// ClientService реализует клиентскую логику: подключение, получение событий, фильтрация и сохранение.
type ClientService struct {
	serverURL    string
	conn         *websocket.Conn
	repo         domain.EventRepository
	logger       *slog.Logger
	receivedIDs  map[string]struct{}
	mu           sync.Mutex
	reconnecting bool
	done         chan struct{}
}

// NewClientService создаёт нового клиента, подключается к серверу и инициализирует репозиторий.
func NewClientService(serverURL, dbPath string, logger *slog.Logger) (*ClientService, error) {
	repo, err := repository.NewSQLiteRepository(dbPath)
	if err != nil {
		return nil, err
	}
	if err := repo.Init(); err != nil {
		return nil, err
	}
	cs := &ClientService{
		serverURL:   serverURL,
		repo:        repo,
		logger:      logger,
		receivedIDs: make(map[string]struct{}),
		done:        make(chan struct{}),
	}
	if err := cs.connect(); err != nil {
		return nil, err
	}
	return cs, nil
}

// connect устанавливает WebSocket-соединение с сервером.
func (cs *ClientService) connect() error {
	u, err := url.Parse(cs.serverURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	cs.conn = conn
	cs.logger.Info("Подключение к серверу установлено", "url", cs.serverURL)
	return nil
}

// Listen запускает цикл получения сообщений с автоматическим переподключением.
func (cs *ClientService) Listen(ctx context.Context) {
	go cs.listenLoop(ctx)
	<-cs.done
}

func (cs *ClientService) listenLoop(ctx context.Context) {
	defer close(cs.done)
	for {
		select {
		case <-ctx.Done():
			cs.logger.Info("Завершение работы клиента по сигналу контекста")
			return
		default:
			_, message, err := cs.conn.ReadMessage()
			if err != nil {
				cs.logger.Error("Ошибка чтения", "error", err)
				cs.reconnect(ctx)
				continue
			}
			var event domain.Event
			if err := json.Unmarshal(message, &event); err != nil {
				cs.logger.Error("Ошибка декодирования JSON", "error", err)
				continue
			}
			cs.processEvent(event)
		}
	}
}

// processEvent фильтрует дубликаты и сохраняет событие.
func (cs *ClientService) processEvent(event domain.Event) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if _, exists := cs.receivedIDs[event.ID]; exists {
		cs.logger.Info("Дублирующее событие отфильтровано", "id", event.ID)
		return
	}
	cs.receivedIDs[event.ID] = struct{}{}
	cs.logger.Info("Обработка события", "event", event)
	if err := cs.repo.Save(event); err != nil {
		cs.logger.Error("Ошибка сохранения события", "error", err)
	}
}

// reconnect пытается восстановить соединение с экспоненциальной задержкой.
func (cs *ClientService) reconnect(ctx context.Context) {
	cs.mu.Lock()
	if cs.reconnecting {
		cs.mu.Unlock()
		return
	}
	cs.reconnecting = true
	cs.mu.Unlock()

	cs.logger.Info("Попытка переподключения...")
	if cs.conn != nil {
		cs.conn.Close()
	}

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			cs.logger.Info("Переподключение отменено (контекст)")
			return
		default:
			err := cs.connect()
			if err == nil {
				cs.mu.Lock()
				cs.reconnecting = false
				cs.mu.Unlock()
				cs.logger.Info("Переподключение успешно")
				return
			}
			cs.logger.Error("Не удалось переподключиться", "error", err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}
