package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	ConversationID string    `json:"conversation_id"`
	ProjectPath    string    `json:"project_path"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Notes          string    `json:"notes"`
}

type Store struct {
	db *sql.DB
}

func InitDB(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database at %s: %w", dbPath, err)
	}

	// Enable WAL mode for better concurrency and durability
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to configure sqlite PRAGMA: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		conversation_id TEXT NOT NULL UNIQUE,
		project_path TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		notes TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_updated_at ON sessions(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_sessions_conversation_id ON sessions(conversation_id);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) SaveSession(name, conversationID, projectPath, notes string) (*Session, error) {
	name = strings.TrimSpace(name)
	conversationID = strings.TrimSpace(conversationID)
	projectPath = strings.TrimSpace(projectPath)

	if conversationID == "" {
		return nil, fmt.Errorf("conversation ID cannot be empty")
	}
	if name == "" {
		name = fmt.Sprintf("Session %s", conversationID[:min(8, len(conversationID))])
	}

	now := time.Now().UTC()

	// Check if already exists
	var existingID int64
	err := s.db.QueryRow("SELECT id FROM sessions WHERE conversation_id = ?", conversationID).Scan(&existingID)
	if err == nil {
		// Update existing session
		_, updateErr := s.db.Exec(`
			UPDATE sessions 
			SET name = ?, project_path = ?, updated_at = ?, notes = ?
			WHERE id = ?
		`, name, projectPath, now, notes, existingID)
		if updateErr != nil {
			return nil, fmt.Errorf("failed to update session: %w", updateErr)
		}
		return s.GetSessionByID(existingID)
	}

	// Insert new session
	res, insertErr := s.db.Exec(`
		INSERT INTO sessions (name, conversation_id, project_path, created_at, updated_at, notes)
		VALUES (?, ?, ?, ?, ?, ?)
	`, name, conversationID, projectPath, now, now, notes)
	if insertErr != nil {
		return nil, fmt.Errorf("failed to insert session: %w", insertErr)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetSessionByID(id)
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`
		SELECT id, name, conversation_id, project_path, created_at, updated_at, notes
		FROM sessions
		ORDER BY updated_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(
			&sess.ID,
			&sess.Name,
			&sess.ConversationID,
			&sess.ProjectPath,
			&sess.CreatedAt,
			&sess.UpdatedAt,
			&sess.Notes,
		); err != nil {
			return nil, fmt.Errorf("failed to scan session row: %w", err)
		}
		sessions = append(sessions, sess)
	}

	return sessions, rows.Err()
}

func (s *Store) GetSessionByID(id int64) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`
		SELECT id, name, conversation_id, project_path, created_at, updated_at, notes
		FROM sessions
		WHERE id = ?
	`, id).Scan(
		&sess.ID,
		&sess.Name,
		&sess.ConversationID,
		&sess.ProjectPath,
		&sess.CreatedAt,
		&sess.UpdatedAt,
		&sess.Notes,
	)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) GetSessionByConversationID(convoID string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(`
		SELECT id, name, conversation_id, project_path, created_at, updated_at, notes
		FROM sessions
		WHERE conversation_id = ?
	`, strings.TrimSpace(convoID)).Scan(
		&sess.ID,
		&sess.Name,
		&sess.ConversationID,
		&sess.ProjectPath,
		&sess.CreatedAt,
		&sess.UpdatedAt,
		&sess.Notes,
	)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) DeleteSession(convoID string) error {
	_, err := s.db.Exec("DELETE FROM sessions WHERE conversation_id = ?", strings.TrimSpace(convoID))
	return err
}

func (s *Store) RenameSession(convoID string, newName string) error {
	_, err := s.db.Exec("UPDATE sessions SET name = ?, updated_at = ? WHERE conversation_id = ?",
		strings.TrimSpace(newName), time.Now().UTC(), strings.TrimSpace(convoID))
	return err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
