package config

import (
	"encoding/json"
	"os"
)

// ServerConfig содержит настройки сервера.
type ServerConfig struct {
	ServerAddr string `json:"server_addr"` // например, ":8080"
	WSPath     string `json:"ws_path"`     // например, "/ws"
	LogLevel   string `json:"log_level"`   // например, "INFO"
}

// ClientConfig содержит настройки клиента.
type ClientConfig struct {
	ClientServerURL string `json:"client_server_url"` // например, "ws://localhost:8080/ws"
	DBPath          string `json:"db_path"`           // например, "client.db"
	NumClients      int    `json:"num_clients"`       // количество одновременно запускаемых клиентов
	LogLevel        string `json:"log_level"`         // например, "INFO"
}

// LoadServerConfig загружает конфигурацию сервера из файла.
func LoadServerConfig(path string) (*ServerConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &ServerConfig{}
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// LoadClientConfig загружает конфигурацию клиента из файла.
func LoadClientConfig(path string) (*ClientConfig, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := &ClientConfig{}
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
