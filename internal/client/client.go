package client

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/url"
	_ "os"
	"sync"
	"time"

	"log/slog"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
)

// Event представляет событие, получаемое от сервера.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// EventRepository определяет интерфейс для сохранения событий.
type EventRepository interface {
	Init() error
	Save(event Event) error
}

// SQLiteRepository реализует сохранение событий в SQLite.
type SQLiteRepository struct {
	db *sql.DB
}

// NewSQLiteRepository создаёт новый репозиторий для базы данных по указанному пути.
func NewSQLiteRepository(dbPath string) (*SQLiteRepository, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return &SQLiteRepository{db: db}, nil
}

// Init создаёт таблицу для хранения событий.
func (repo *SQLiteRepository) Init() error {
	query := `
        CREATE TABLE IF NOT EXISTS events (
            id TEXT PRIMARY KEY,
            type TEXT,
            message TEXT,
            timestamp DATETIME
        );
    `
	_, err := repo.db.Exec(query)
	return err
}

// Save сохраняет событие, если его там ещё нет.
func (repo *SQLiteRepository) Save(event Event) error {
	query := `INSERT OR IGNORE INTO events (id, type, message, timestamp) VALUES (?, ?, ?, ?);`
	_, err := repo.db.Exec(query, event.ID, event.Type, event.Message, event.Timestamp)
	return err
}

// Client реализует логику подключения к серверу, фильтрации дубликатов и сохранения событий.
type Client struct {
	serverURL   string
	conn        *websocket.Conn
	repo        EventRepository
	logger      *slog.Logger
	receivedIDs map[string]struct{}
	mu          sync.Mutex
	// Флаг переподключения.
	reconnecting bool
	done         chan struct{}
}

// NewClient создаёт нового клиента с подключением к серверу и инициализацией репозитория.
func NewClient(serverURL, dbPath string, logger *slog.Logger) (*Client, error) {
	repo, err := NewSQLiteRepository(dbPath)
	if err != nil {
		return nil, err
	}
	if err := repo.Init(); err != nil {
		return nil, err
	}
	c := &Client{
		serverURL:   serverURL,
		repo:        repo,
		logger:      logger,
		receivedIDs: make(map[string]struct{}),
		done:        make(chan struct{}),
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

// connect устанавливает WebSocket-соединение с сервером.
func (c *Client) connect() error {
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	c.conn = conn
	c.logger.Info("Подключение к серверу установлено", "url", c.serverURL)
	return nil
}

// Listen запускает цикл получения сообщений от сервера с автоматическим переподключением.
func (c *Client) Listen(ctx context.Context) {
	go c.listenLoop(ctx)
	<-c.done
}

func (c *Client) listenLoop(ctx context.Context) {
	defer close(c.done)
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Завершение работы клиента по сигналу контекста")
			return
		default:
			_, message, err := c.conn.ReadMessage()
			if err != nil {
				c.logger.Error("Ошибка чтения", "error", err)
				c.reconnect(ctx)
				continue
			}
			var event Event
			if err := json.Unmarshal(message, &event); err != nil {
				c.logger.Error("Ошибка декодирования JSON", "error", err)
				continue
			}
			c.processEvent(event)
		}
	}
}

// processEvent фильтрует дубликаты и сохраняет уникальное событие.
func (c *Client) processEvent(event Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.receivedIDs[event.ID]; exists {
		c.logger.Info("Дублирующее событие отфильтровано", "id", event.ID)
		return
	}
	c.receivedIDs[event.ID] = struct{}{}
	c.logger.Info("Обработка события", "event", event)
	if err := c.repo.Save(event); err != nil {
		c.logger.Error("Ошибка сохранения события", "error", err)
	}
}

// reconnect пытается восстановить соединение с сервером с экспоненциальной задержкой.
func (c *Client) reconnect(ctx context.Context) {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	c.logger.Info("Попытка переподключения...")
	// Закрываем существующее соединение.
	if c.conn != nil {
		c.conn.Close()
	}

	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Переподключение отменено (контекст)")
			return
		default:
			err := c.connect()
			if err == nil {
				c.mu.Lock()
				c.reconnecting = false
				c.mu.Unlock()
				c.logger.Info("Переподключение успешно")
				return
			}
			c.logger.Error("Не удалось переподключиться", "error", err)
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}
