package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/sidharth/agy-session-helper/internal/antigravity"
	"github.com/sidharth/agy-session-helper/internal/config"
	"github.com/sidharth/agy-session-helper/internal/db"
	"github.com/sidharth/agy-session-helper/internal/runner"
)

// StartInteractiveBrowser launches the terminal session selector.
func StartInteractiveBrowser(cfg *config.Config, store *db.Store) error {
	for {
		sessions, err := store.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to load sessions: %w", err)
		}

		if len(sessions) == 0 {
			action, err := showEmptyState(cfg, store)
			if err != nil || action == "quit" {
				return err
			}
			if action == "new" {
				return launchNewSession(cfg, store, "")
			}
			continue
		}

		// Check if terminal is raw-capable
		fd := int(os.Stdin.Fd())
		if !term.IsTerminal(fd) {
			return showLineModeBrowser(cfg, store, sessions)
		}

		action, selectedSess, err := runRawModeBrowser(sessions)
		if err != nil {
			// Fallback to line mode on error
			return showLineModeBrowser(cfg, store, sessions)
		}

		switch action {
		case "open":
			if selectedSess != nil {
				return launchSession(cfg, store, selectedSess)
			}
		case "new":
			return launchNewSession(cfg, store, "")
		case "delete":
			if selectedSess != nil {
				promptDeleteInBrowser(cfg, store, selectedSess)
			}
		case "rename":
			if selectedSess != nil {
				promptRenameInBrowser(store, selectedSess)
			}
		case "import":
			importFromAgy(cfg, store)
		case "quit":
			return nil
		}
	}
}

func showEmptyState(cfg *config.Config, store *db.Store) (string, error) {
	fmt.Println()
	fmt.Println("\033[1;36m┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓\033[0m")
	fmt.Println("\033[1;36m┃                 Antigravity CLI Session Helper                  ┃\033[0m")
	fmt.Println("\033[1;36m┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛\033[0m")
	fmt.Println()
	fmt.Println("  \033[33mNo saved sessions found in helper database.\033[0m")
	fmt.Println()
	fmt.Println("  \033[1m[1]\033[0m Launch a new Antigravity session in current directory")
	fmt.Println("  \033[1m[2]\033[0m Import existing sessions from Antigravity CLI history")
	fmt.Println("  \033[1m[q]\033[0m Quit")
	fmt.Println()
	fmt.Print("  \033[1mChoose an option: \033[0m")

	reader := bufio.NewReader(os.Stdin)
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "1", "n", "new":
		return "new", nil
	case "2", "i", "import":
		importFromAgy(cfg, store)
		return "refresh", nil
	default:
		return "quit", nil
	}
}

func runRawModeBrowser(sessions []db.Session) (string, *db.Session, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return "", nil, err
	}
	defer term.Restore(fd, oldState)

	selectedIndex := 0
	filterText := ""
	isFiltering := false

	for {
		filtered := filterSessions(sessions, filterText)
		if selectedIndex >= len(filtered) {
			selectedIndex = max(0, len(filtered)-1)
		}

		renderScreen(filtered, selectedIndex, filterText, isFiltering)

		// Read key
		buf := make([]byte, 3)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "quit", nil, err
		}

		if n == 1 {
			b := buf[0]
			if isFiltering {
				switch b {
				case 13: // Enter
					isFiltering = false
				case 27: // Esc
					isFiltering = false
					filterText = ""
				case 127, 8: // Backspace
					if len(filterText) > 0 {
						filterText = filterText[:len(filterText)-1]
					}
				case 3: // Ctrl+C
					return "quit", nil, nil
				default:
					if b >= 32 && b <= 126 {
						filterText += string(b)
					}
				}
				continue
			}

			// Normal Navigation Mode
			switch b {
			case 3, 'q', 'Q': // Ctrl+C or q
				return "quit", nil, nil
			case 13: // Enter
				if len(filtered) > 0 {
					return "open", &filtered[selectedIndex], nil
				}
			case 'k', 'K': // Up
				if selectedIndex > 0 {
					selectedIndex--
				}
			case 'j', 'J': // Down
				if selectedIndex < len(filtered)-1 {
					selectedIndex++
				}
			case 'n', 'N': // New session
				return "new", nil, nil
			case 'd', 'D': // Delete session
				if len(filtered) > 0 {
					return "delete", &filtered[selectedIndex], nil
				}
			case 'r', 'R': // Rename
				if len(filtered) > 0 {
					return "rename", &filtered[selectedIndex], nil
				}
			case 'i', 'I': // Import
				return "import", nil, nil
			case '/': // Search
				isFiltering = true
			}
		} else if n == 3 && buf[0] == 27 && buf[1] == 91 {
			// ANSI escape sequences
			switch buf[2] {
			case 65: // Up Arrow
				if selectedIndex > 0 {
					selectedIndex--
				}
			case 66: // Down Arrow
				if selectedIndex < len(filtered)-1 {
					selectedIndex++
				}
			}
		}
	}
}

func renderScreen(sessions []db.Session, selected int, filter string, isFiltering bool) {
	// Clear screen & home cursor
	fmt.Print("\033[2J\033[H")

	fmt.Print("\033[1;36m┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓\r\n\033[0m")
	fmt.Print("\033[1;36m┃                         ANTIGRAVITY CLI SESSION HELPER                           ┃\r\n\033[0m")
	fmt.Print("\033[1;36m┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛\r\n\033[0m")

	if isFiltering {
		fmt.Printf(" \033[1;33m[Search]:\033[0m %s\033[5m█\033[0m (Press Enter to finish, Esc to clear)\r\n\r\n", filter)
	} else if filter != "" {
		fmt.Printf(" \033[1;33m[Filtered by]:\033[0m %s (Press '/' to edit, 'Esc' in search to clear)\r\n\r\n", filter)
	} else {
		fmt.Print(" \033[2mSelect a session to resume with Antigravity CLI\033[0m\r\n\r\n")
	}

	// Table header
	fmt.Print("   \033[1;4m#   SESSION NAME                   PROJECT WORKSPACE              LAST ACTIVE\033[0m\r\n")

	if len(sessions) == 0 {
		fmt.Print("\r\n    \033[33mNo sessions matching filter.\033[0m\r\n\r\n")
	} else {
		for i, s := range sessions {
			cursor := "  "
			colorPrefix := ""
			colorSuffix := "\033[0m"

			if i == selected {
				cursor = "\033[1;32m❯ \033[0m"
				colorPrefix = "\033[1;32m"
			}

			name := truncateStr(s.Name, 30)
			path := formatPath(s.ProjectPath, 30)
			relTime := formatRelativeTime(s.UpdatedAt)

			fmt.Printf("%s%s%-3d %-30s %-30s %-12s%s\r\n",
				cursor, colorPrefix, i+1, name, path, relTime, colorSuffix)
		}
	}

	fmt.Print("\r\n\033[2m──────────────────────────────────────────────────────────────────────────────────\033[0m\r\n")
	fmt.Print(" \033[1m[Enter]\033[0m Open   \033[1m[n]\033[0m New Session   \033[1m[d]\033[0m Delete   \033[1m[r]\033[0m Rename   \033[1m[/]\033[0m Search   \033[1m[q]\033[0m Quit\r\n")
}

func filterSessions(sessions []db.Session, query string) []db.Session {
	if query == "" {
		return sessions
	}
	q := strings.ToLower(query)
	var res []db.Session
	for _, s := range sessions {
		if strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.ProjectPath), q) ||
			strings.Contains(strings.ToLower(s.ConversationID), q) {
			res = append(res, s)
		}
	}
	return res
}

func showLineModeBrowser(cfg *config.Config, store *db.Store, sessions []db.Session) error {
	fmt.Println()
	fmt.Println("\033[1;36m=== Antigravity CLI Saved Sessions ===\033[0m")
	fmt.Println()
	fmt.Printf("%-4s %-30s %-35s %-15s\n", "#", "SESSION NAME", "PROJECT WORKSPACE", "LAST ACTIVE")
	fmt.Println(strings.Repeat("-", 85))

	for i, s := range sessions {
		name := truncateStr(s.Name, 28)
		path := formatPath(s.ProjectPath, 33)
		relTime := formatRelativeTime(s.UpdatedAt)
		fmt.Printf("%-4d %-30s %-35s %-15s\n", i+1, name, path, relTime)
	}

	fmt.Println(strings.Repeat("-", 85))
	fmt.Print("\033[1mEnter session # to open, 'n' for new, 'd #' to delete, 'q' to quit: \033[0m")

	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)

	if line == "q" || line == "quit" || line == "" {
		return nil
	}
	if line == "n" || line == "new" {
		return launchNewSession(cfg, store, "")
	}

	var idx int
	if _, err := fmt.Sscanf(line, "%d", &idx); err == nil && idx >= 1 && idx <= len(sessions) {
		return launchSession(cfg, store, &sessions[idx-1])
	}

	return nil
}

func launchSession(cfg *config.Config, store *db.Store, sess *db.Session) error {
	targetDir := sess.ProjectPath
	if targetDir == "" {
		targetDir = "."
	}

	// Verify project directory exists
	if fi, err := os.Stat(targetDir); err != nil || !fi.IsDir() {
		fmt.Printf("\033[33mWarning: Project path '%s' does not exist.\033[0m\n", targetDir)
		fmt.Print("Open in current working directory instead? (y/n): ")
		reader := bufio.NewReader(os.Stdin)
		ans, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(ans)), "y") {
			return nil
		}
		targetDir, _ = os.Getwd()
	}

	fmt.Println()
	fmt.Printf("\033[1;32m🚀 Resuming Antigravity session: %s\033[0m\n", sess.Name)
	fmt.Printf("   Workspace: %s\n", targetDir)
	fmt.Printf("   Command:   \033[36magy --conversation=%s\033[0m\n\n", sess.ConversationID)

	args := []string{"--conversation=" + sess.ConversationID}
	return runner.RunAgy(cfg, store, args, targetDir)
}

func launchNewSession(cfg *config.Config, store *db.Store, dir string) error {
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			dir = "."
		}
	}

	fmt.Println()
	fmt.Printf("\033[1;32m🚀 Starting new Antigravity session in %s\033[0m\n\n", dir)
	return runner.RunAgy(cfg, store, nil, dir)
}

func promptDeleteInBrowser(cfg *config.Config, store *db.Store, sess *db.Session) {
	fmt.Println()
	fmt.Printf("\033[1;31mDelete session '%s' (%s)?\033[0m\n", sess.Name, sess.ConversationID)
	fmt.Print("Also remove conversation data from Antigravity CLI? (y/n/cancel): ")
	reader := bufio.NewReader(os.Stdin)
	ans, _ := reader.ReadString('\n')
	ans = strings.ToLower(strings.TrimSpace(ans))

	if ans == "cancel" || ans == "c" {
		return
	}

	_ = store.DeleteSession(sess.ConversationID)
	if ans == "y" || ans == "yes" {
		_ = antigravity.DeleteAgySession(cfg.AgyAppDataDir, sess.ConversationID)
		fmt.Println("\033[1;32m✓ Session and Antigravity conversation removed.\033[0m")
	} else {
		fmt.Println("\033[1;32m✓ Removed from helper database (Antigravity files kept).\033[0m")
	}
	time.Sleep(1 * time.Second)
}

func promptRenameInBrowser(store *db.Store, sess *db.Session) {
	fmt.Println()
	fmt.Printf("\033[1mRename session '%s'\033[0m\n", sess.Name)
	fmt.Print("Enter new name: ")
	reader := bufio.NewReader(os.Stdin)
	newName, _ := reader.ReadString('\n')
	newName = strings.TrimSpace(newName)
	if newName != "" {
		_ = store.RenameSession(sess.ConversationID, newName)
		fmt.Printf("\033[1;32m✓ Session renamed to '%s'\033[0m\n", newName)
		time.Sleep(800 * time.Millisecond)
	}
}

func importFromAgy(cfg *config.Config, store *db.Store) {
	fmt.Println()
	fmt.Println("\033[1;36mScanning Antigravity CLI conversations...\033[0m")
	convos, err := antigravity.ListAgyRawConversations(cfg.AgyAppDataDir, 25)
	if err != nil {
		fmt.Printf("\033[31mError scanning conversations: %v\033[0m\n", err)
		time.Sleep(1500 * time.Millisecond)
		return
	}

	imported := 0
	for _, c := range convos {
		// Check if exists
		existing, _ := store.GetSessionByConversationID(c.ConversationID)
		if existing == nil {
			title := c.Title
			if title == "" {
				title = fmt.Sprintf("Imported Session %s", c.ConversationID[:8])
			}
			proj := c.ProjectPath
			if proj == "" {
				proj, _ = os.Getwd()
			}
			_, _ = store.SaveSession(title, c.ConversationID, proj, "Imported from Antigravity history")
			imported++
		}
	}

	fmt.Printf("\033[1;32m✓ Imported %d recent conversations into helper database!\033[0m\n", imported)
	time.Sleep(1200 * time.Millisecond)
}

func formatPath(path string, maxLen int) string {
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		path = "~" + strings.TrimPrefix(path, home)
	}
	return truncateStr(path, maxLen)
}

func truncateStr(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		if maxLen <= 3 {
			return s[:maxLen]
		}
		return s[:maxLen-3] + "..."
	}
	return s
}

func formatRelativeTime(t time.Time) string {
	diff := time.Since(t)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm ago", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(diff.Hours()))
	}
	if diff < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(diff.Hours()/24))
	}
	return t.Format("Jan 02")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
