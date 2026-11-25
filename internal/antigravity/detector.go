package antigravity

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var (
	uuidRegex       = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	convoFlagRegex  = regexp.MustCompile(`--(?:conversation|resume)[=\s]+([0-9a-fA-F\-]{36})`)
	shortFlagRegex  = regexp.MustCompile(`(?:^|\s)-r[=\s]+([0-9a-fA-F\-]{36})`)
)

type DetectedSession struct {
	ConversationID string
	Title          string
	ProjectPath    string
	LastModified   time.Time
}

// ExtractFromArgs extracts a conversation ID from CLI arguments if present.
func ExtractFromArgs(args []string) string {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--conversation=") {
			return strings.TrimPrefix(arg, "--conversation=")
		}
		if strings.HasPrefix(arg, "--resume=") {
			return strings.TrimPrefix(arg, "--resume=")
		}
		if (arg == "--conversation" || arg == "--resume" || arg == "-r") && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(arg, "-r=") {
			return strings.TrimPrefix(arg, "-r=")
		}
	}
	return ""
}

// ExtractFromText finds the conversation continue string like:
// "agy --conversation=0fa98ade-3060-4546-ada4-0652329fdfb6"
func ExtractFromText(text string) string {
	matches := convoFlagRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	matches = shortFlagRegex.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// DetectRecentConversation finds the conversation ID and title for a session in projectPath
// that was active around or after startTime.
func DetectRecentConversation(agyDataDir, projectPath string, startTime time.Time) (*DetectedSession, error) {
	// Priority 1: Check conversation_summaries.db (the most authoritative source)
	if sess, err := detectFromSummariesDB(agyDataDir, projectPath, startTime); err == nil && sess != nil {
		return sess, nil
	}

	// Priority 2: Check history.jsonl
	if sess, err := detectFromHistory(agyDataDir, projectPath, startTime); err == nil && sess != nil {
		return sess, nil
	}

	// Priority 3: Check cli.log
	if sess, err := detectFromLog(agyDataDir, startTime); err == nil && sess != nil {
		return sess, nil
	}

	// Priority 4: Check file modification times in conversations/ directory
	if sess, err := detectFromFiles(agyDataDir, projectPath, startTime); err == nil && sess != nil {
		return sess, nil
	}

	return nil, fmt.Errorf("no active conversation detected")
}

func detectFromSummariesDB(agyDataDir, projectPath string, startTime time.Time) (*DetectedSession, error) {
	dbPath := filepath.Join(agyDataDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
		SELECT conversation_id, title, workspace_uris, datetime(last_modified_time)
		FROM conversation_summaries
		ORDER BY last_modified_time DESC
		LIMIT 15
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cleanProjPath := filepath.Clean(projectPath)
	expectedURI := "file://" + cleanProjPath

	var fallback *DetectedSession

	for rows.Next() {
		var id, title, urisRaw, modTimeStr string
		if err := rows.Scan(&id, &title, &urisRaw, &modTimeStr); err != nil {
			continue
		}

		sess := &DetectedSession{
			ConversationID: id,
			Title:          title,
			ProjectPath:    cleanProjPath,
		}
		if t, err := time.Parse("2006-01-02 15:04:05", modTimeStr); err == nil {
			sess.LastModified = t
		}

		// Check if workspace matches
		if strings.Contains(urisRaw, expectedURI) || strings.Contains(urisRaw, cleanProjPath) {
			return sess, nil
		}

		// Also extract path from urisRaw if present
		if fallback == nil {
			var uris []string
			if err := json.Unmarshal([]byte(urisRaw), &uris); err == nil && len(uris) > 0 {
				if parsed, err := url.Parse(uris[0]); err == nil && parsed.Path != "" {
					sess.ProjectPath = parsed.Path
				}
			}
			fallback = sess
		}
	}

	return fallback, nil
}

func detectFromHistory(agyDataDir, projectPath string, startTime time.Time) (*DetectedSession, error) {
	historyPath := filepath.Join(agyDataDir, "history.jsonl")
	f, err := os.Open(historyPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read last 100 lines
	lines := make([]string, 0, 100)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > 200 {
			lines = lines[1:]
		}
	}

	cleanProjPath := filepath.Clean(projectPath)

	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		var entry struct {
			ConversationID string `json:"conversationId"`
			Workspace      string `json:"workspace"`
			Display        string `json:"display"`
			Timestamp      int64  `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err == nil && entry.ConversationID != "" {
			if cleanProjPath != "" && filepath.Clean(entry.Workspace) == cleanProjPath {
				return &DetectedSession{
					ConversationID: entry.ConversationID,
					Title:          entry.Display,
					ProjectPath:    entry.Workspace,
					LastModified:   time.UnixMilli(entry.Timestamp),
				}, nil
			}
		}
	}

	// If no exact workspace match, return the latest one found
	for i := len(lines) - 1; i >= 0; i-- {
		var entry struct {
			ConversationID string `json:"conversationId"`
			Workspace      string `json:"workspace"`
			Display        string `json:"display"`
			Timestamp      int64  `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(lines[i]), &entry); err == nil && entry.ConversationID != "" {
			return &DetectedSession{
				ConversationID: entry.ConversationID,
				Title:          entry.Display,
				ProjectPath:    entry.Workspace,
				LastModified:   time.UnixMilli(entry.Timestamp),
			}, nil
		}
	}

	return nil, fmt.Errorf("no history entry found")
}

func detectFromLog(agyDataDir string, startTime time.Time) (*DetectedSession, error) {
	logPath := filepath.Join(agyDataDir, "cli.log")
	f, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Look backwards for Created/Streaming/Resuming conversation <uuid>
	re := regexp.MustCompile(`(?:Created conversation|Streaming conversation|Resuming conversation)\s+([0-9a-fA-F\-]{36})`)
	scanner := bufio.NewScanner(f)
	var lastID string
	for scanner.Scan() {
		line := scanner.Text()
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			lastID = matches[1]
		}
	}

	if lastID != "" {
		return &DetectedSession{
			ConversationID: lastID,
			LastModified:   time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no conversation found in log")
}

func detectFromFiles(agyDataDir, projectPath string, startTime time.Time) (*DetectedSession, error) {
	convDir := filepath.Join(agyDataDir, "conversations")
	entries, err := os.ReadDir(convDir)
	if err != nil {
		return nil, err
	}

	type fileInfo struct {
		id      string
		modTime time.Time
	}
	var files []fileInfo

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".db") {
			id := strings.TrimSuffix(entry.Name(), ".db")
			if uuidRegex.MatchString(id) {
				if info, err := entry.Info(); err == nil {
					files = append(files, fileInfo{id: id, modTime: info.ModTime()})
				}
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no conversation databases found")
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].modTime.After(files[j].modTime)
	})

	return &DetectedSession{
		ConversationID: files[0].id,
		ProjectPath:    projectPath,
		LastModified:   files[0].modTime,
	}, nil
}

// DeleteAgySession completely wipes the conversation from Antigravity CLI storage.
func DeleteAgySession(agyDataDir, conversationID string) error {
	id := strings.TrimSpace(conversationID)
	if !uuidRegex.MatchString(id) {
		return fmt.Errorf("invalid conversation ID: %s", conversationID)
	}

	var errorsOccurred []string

	// 1. Remove conversation databases
	convDir := filepath.Join(agyDataDir, "conversations")
	for _, ext := range []string{".db", ".db-shm", ".db-wal"} {
		fPath := filepath.Join(convDir, id+ext)
		if err := os.Remove(fPath); err != nil && !os.IsNotExist(err) {
			errorsOccurred = append(errorsOccurred, err.Error())
		}
	}

	// 2. Remove brain directory
	brainDir := filepath.Join(agyDataDir, "brain", id)
	if err := os.RemoveAll(brainDir); err != nil && !os.IsNotExist(err) {
		errorsOccurred = append(errorsOccurred, err.Error())
	}

	// 3. Remove annotations / presence if present
	for _, sub := range []string{"annotations", "presence"} {
		subDir := filepath.Join(agyDataDir, sub, id)
		_ = os.RemoveAll(subDir)
	}

	// 4. Delete entries from conversation_summaries.db
	summariesDB := filepath.Join(agyDataDir, "conversation_summaries.db")
	if db, err := sql.Open("sqlite", summariesDB); err == nil {
		defer db.Close()
		_, _ = db.Exec("DELETE FROM conversation_summaries WHERE conversation_id = ?", id)
		_, _ = db.Exec("DELETE FROM conversations_fts WHERE conversation_id = ?", id)
		_, _ = db.Exec("DELETE FROM conversation_parents WHERE conversation_id = ?", id)
	}

	if len(errorsOccurred) > 0 {
		return fmt.Errorf("some files could not be removed: %s", strings.Join(errorsOccurred, "; "))
	}
	return nil
}

// ListAgyRawConversations reads recent conversations directly from Antigravity's store for easy importing.
func ListAgyRawConversations(agyDataDir string, limit int) ([]DetectedSession, error) {
	dbPath := filepath.Join(agyDataDir, "conversation_summaries.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if limit <= 0 {
		limit = 30
	}

	rows, err := db.Query(`
		SELECT conversation_id, title, workspace_uris, datetime(last_modified_time)
		FROM conversation_summaries
		ORDER BY last_modified_time DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []DetectedSession
	for rows.Next() {
		var id, title, urisRaw, modTimeStr string
		if err := rows.Scan(&id, &title, &urisRaw, &modTimeStr); err != nil {
			continue
		}

		projPath := ""
		var uris []string
		if err := json.Unmarshal([]byte(urisRaw), &uris); err == nil && len(uris) > 0 {
			if parsed, err := url.Parse(uris[0]); err == nil && parsed.Path != "" {
				projPath = parsed.Path
			} else {
				projPath = strings.TrimPrefix(uris[0], "file://")
			}
		}

		sess := DetectedSession{
			ConversationID: id,
			Title:          title,
			ProjectPath:    projPath,
		}
		if t, err := time.Parse("2006-01-02 15:04:05", modTimeStr); err == nil {
			sess.LastModified = t
		}
		list = append(list, sess)
	}

	return list, nil
}
