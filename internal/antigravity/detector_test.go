package antigravity

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestExtractFromArgs(t *testing.T) {
	tests := []struct {
		args     []string
		expected string
	}{
		{[]string{"--conversation=0fa98ade-3060-4546-ada4-0652329fdfb6"}, "0fa98ade-3060-4546-ada4-0652329fdfb6"},
		{[]string{"--resume=0fa98ade-3060-4546-ada4-0652329fdfb6"}, "0fa98ade-3060-4546-ada4-0652329fdfb6"},
		{[]string{"-r", "0fa98ade-3060-4546-ada4-0652329fdfb6"}, "0fa98ade-3060-4546-ada4-0652329fdfb6"},
		{[]string{"--conversation", "0fa98ade-3060-4546-ada4-0652329fdfb6"}, "0fa98ade-3060-4546-ada4-0652329fdfb6"},
		{[]string{"--print", "hello"}, ""},
	}

	for _, tt := range tests {
		got := ExtractFromArgs(tt.args)
		if got != tt.expected {
			t.Errorf("ExtractFromArgs(%v) = %q; want %q", tt.args, got, tt.expected)
		}
	}
}

func TestExtractFromText(t *testing.T) {
	text := "To continue this conversation, run: agy --conversation=0fa98ade-3060-4546-ada4-0652329fdfb6"
	expected := "0fa98ade-3060-4546-ada4-0652329fdfb6"
	got := ExtractFromText(text)
	if got != expected {
		t.Errorf("ExtractFromText() = %q; want %q", got, expected)
	}
}

func TestDetectFromSummariesDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agy-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "conversation_summaries.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}

	_, err = db.Exec(`
		CREATE TABLE conversation_summaries (
			conversation_id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			workspace_uris TEXT NOT NULL,
			last_modified_time DATETIME NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	projectPath := "/Users/test/my-project"
	convoID := "11111111-2222-3333-4444-555555555555"
	_, err = db.Exec(`
		INSERT INTO conversation_summaries (conversation_id, title, workspace_uris, last_modified_time)
		VALUES (?, ?, ?, ?);
	`, convoID, "Test Session Title", `["file:///Users/test/my-project"]`, "2026-11-25 10:00:00")
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}
	db.Close()

	sess, err := DetectRecentConversation(tmpDir, projectPath, time.Now().Add(-1*time.Hour))
	if err != nil {
		t.Fatalf("DetectRecentConversation failed: %v", err)
	}
	if sess.ConversationID != convoID {
		t.Errorf("expected %s, got %s", convoID, sess.ConversationID)
	}
	if sess.Title != "Test Session Title" {
		t.Errorf("expected 'Test Session Title', got '%s'", sess.Title)
	}
}

func TestDeleteAgySession(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agy-delete-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	convDir := filepath.Join(tmpDir, "conversations")
	brainDir := filepath.Join(tmpDir, "brain")
	_ = os.MkdirAll(convDir, 0755)
	_ = os.MkdirAll(brainDir, 0755)

	convoID := "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	dbFile := filepath.Join(convDir, convoID+".db")
	_ = os.WriteFile(dbFile, []byte("fake db"), 0644)

	sessBrain := filepath.Join(brainDir, convoID)
	_ = os.MkdirAll(sessBrain, 0755)
	_ = os.WriteFile(filepath.Join(sessBrain, "notes.md"), []byte("# Notes"), 0644)

	// Also create summaries DB
	sumDbPath := filepath.Join(tmpDir, "conversation_summaries.db")
	sumDb, _ := sql.Open("sqlite", sumDbPath)
	_, _ = sumDb.Exec("CREATE TABLE conversation_summaries (conversation_id TEXT);")
	_, _ = sumDb.Exec("INSERT INTO conversation_summaries VALUES (?);", convoID)
	sumDb.Close()

	if err := DeleteAgySession(tmpDir, convoID); err != nil {
		t.Fatalf("DeleteAgySession failed: %v", err)
	}

	if _, err := os.Stat(dbFile); !os.IsNotExist(err) {
		t.Errorf("conversation db file still exists")
	}
	if _, err := os.Stat(sessBrain); !os.IsNotExist(err) {
		t.Errorf("brain directory still exists")
	}
}
