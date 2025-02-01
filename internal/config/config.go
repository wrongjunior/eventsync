package config

import (
	"encoding/json"
	"os"
)

// Config содержит настройки для сервера и клиента.
type Config struct {
	ServerAddr      string `json:"server_addr"`       // адрес HTTP-сервера (например, ":8080")
	WSPath          string `json:"ws_path"`           // путь WebSocket (например, "/ws")
	DBPath          string `json:"db_path"`           // путь к SQLite БД (например, "client.db")
	LogLevel        string `json:"log_level"`         // уровень логирования (например, "INFO")
	ClientServerURL string `json:"client_server_url"` // URL для подключения клиента (например, "ws://localhost:8080/ws")
}

// LoadConfig загружает конфигурацию из указанного JSON-файла.
func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &Config{}
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
