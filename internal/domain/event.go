package domain

import "time"

// Event представляет событие, генерируемое сервером и обрабатываемое клиентом.
type Event struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}
