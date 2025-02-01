package client

import (
	"context"
	"encoding/json"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wrongjunior/eventsync/internal/domain"
	"github.com/wrongjunior/eventsync/internal/service"
	"log/slog"
)

// ClientTransport реализует транспортный слой клиента: подключение, получение сообщений и переподключение.
type ClientTransport struct {
	ServerURL     string
	Conn          *websocket.Conn
	Logger        *slog.Logger
	ClientService *service.ClientService
	reconnecting  bool
}

// NewClientTransport создаёт новый экземпляр транспорта клиента.
func NewClientTransport(serverURL string, cs *service.ClientService, logger *slog.Logger) *ClientTransport {
	return &ClientTransport{
		ServerURL:     serverURL,
		ClientService: cs,
		Logger:        logger,
	}
}

// connect устанавливает WebSocket-соединение с сервером.
func (ct *ClientTransport) connect() error {
	u, err := url.Parse(ct.ServerURL)
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}
	ct.Conn = conn
	ct.Logger.Info("Connected to server", "url", ct.ServerURL)
	return nil
}

// Listen запускает цикл получения сообщений с автоматическим переподключением.
func (ct *ClientTransport) Listen(ctx context.Context) {
	// Первоначальное соединение.
	if err := ct.connect(); err != nil {
		ct.Logger.Error("Initial connection failed", "error", err)
		go ct.reconnect(ctx)
	}

	for {
		select {
		case <-ctx.Done():
			ct.Logger.Info("Client transport shutting down")
			return
		default:
			_, message, err := ct.Conn.ReadMessage()
			if err != nil {
				ct.Logger.Error("Read error", "error", err)
				ct.reconnect(ctx)
				continue
			}
			var event domain.Event
			if err := json.Unmarshal(message, &event); err != nil {
				ct.Logger.Error("JSON unmarshal error", "error", err)
				continue
			}
			ct.ClientService.ProcessEvent(event)
		}
	}
}

// reconnect пытается восстановить соединение с экспоненциальной задержкой.
func (ct *ClientTransport) reconnect(ctx context.Context) {
	if ct.reconnecting {
		return
	}
	ct.reconnecting = true
	if ct.Conn != nil {
		ct.Conn.Close()
	}
	backoff := time.Second
	for {
		select {
		case <-ctx.Done():
			ct.Logger.Info("Reconnection cancelled")
			return
		default:
			if err := ct.connect(); err == nil {
				ct.reconnecting = false
				ct.Logger.Info("Reconnected successfully")
				return
			}
			ct.Logger.Error("Reconnection attempt failed", "error", "connection error")
			time.Sleep(backoff)
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
	}
}
