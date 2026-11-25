package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agy-db-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test_sessions.db")
	store, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer store.Close()

	// 1. Test SaveSession
	convoID := "0fa98ade-3060-4546-ada4-0652329fdfb6"
	name := "My First Session"
	projectPath := "/Users/test/projects/my-app"
	notes := "Test notes"

	sess, err := store.SaveSession(name, convoID, projectPath, notes)
	if err != nil {
		t.Fatalf("SaveSession failed: %v", err)
	}

	if sess.Name != name {
		t.Errorf("expected name %s, got %s", name, sess.Name)
	}
	if sess.ConversationID != convoID {
		t.Errorf("expected conversation_id %s, got %s", convoID, sess.ConversationID)
	}
	if sess.ProjectPath != projectPath {
		t.Errorf("expected project_path %s, got %s", projectPath, sess.ProjectPath)
	}

	// 2. Test GetSessionByConversationID
	byConvo, err := store.GetSessionByConversationID(convoID)
	if err != nil {
		t.Fatalf("GetSessionByConversationID failed: %v", err)
	}
	if byConvo.ID != sess.ID {
		t.Errorf("expected ID %d, got %d", sess.ID, byConvo.ID)
	}

	// 3. Test ListSessions
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	// 4. Test Update on duplicate conversation ID
	time.Sleep(10 * time.Millisecond)
	updatedName := "Renamed Session"
	sessUpdated, err := store.SaveSession(updatedName, convoID, projectPath, "updated notes")
	if err != nil {
		t.Fatalf("SaveSession update failed: %v", err)
	}
	if sessUpdated.Name != updatedName {
		t.Errorf("expected updated name %s, got %s", updatedName, sessUpdated.Name)
	}

	sessionsAfterUpdate, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(sessionsAfterUpdate) != 1 {
		t.Errorf("expected still 1 session, got %d", len(sessionsAfterUpdate))
	}

	// 5. Test RenameSession
	if err := store.RenameSession(convoID, "Final Name"); err != nil {
		t.Fatalf("RenameSession failed: %v", err)
	}
	renamed, err := store.GetSessionByConversationID(convoID)
	if err != nil {
		t.Fatalf("GetSessionByConversationID failed: %v", err)
	}
	if renamed.Name != "Final Name" {
		t.Errorf("expected name 'Final Name', got '%s'", renamed.Name)
	}

	// 6. Test DeleteSession
	if err := store.DeleteSession(convoID); err != nil {
		t.Fatalf("DeleteSession failed: %v", err)
	}
	emptySessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions failed: %v", err)
	}
	if len(emptySessions) != 0 {
		t.Errorf("expected 0 sessions after deletion, got %d", len(emptySessions))
	}
}
