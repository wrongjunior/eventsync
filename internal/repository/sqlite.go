package repository

import (
	"database/sql"

	"github.com/wrongjunior/eventsync/internal/domain"
)

// EventRepository определяет интерфейс для сохранения событий.
type EventRepository interface {
	Init() error
	Save(event domain.Event) error
}

// SQLiteRepository реализует сохранение событий в SQLite.
type SQLiteRepository struct {
	DB *sql.DB
}

// NewSQLiteRepository создаёт новый репозиторий для SQLite.
func NewSQLiteRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{DB: db}
}

// Init создаёт таблицу, если она отсутствует.
func (repo *SQLiteRepository) Init() error {
	query := `
        CREATE TABLE IF NOT EXISTS events (
            id TEXT PRIMARY KEY,
            type TEXT,
            message TEXT,
            timestamp DATETIME
        );
    `
	_, err := repo.DB.Exec(query)
	return err
}

// Save сохраняет событие, если его ещё нет в БД.
func (repo *SQLiteRepository) Save(event domain.Event) error {
	query := `INSERT OR IGNORE INTO events (id, type, message, timestamp) VALUES (?, ?, ?, ?);`
	_, err := repo.DB.Exec(query, event.ID, event.Type, event.Message, event.Timestamp)
	return err
}
