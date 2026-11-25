package ui

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/term"

	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/antigravity"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/config"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/db"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/runner"
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

	// Enter alternate screen buffer & hide cursor for zero-flicker rendering
	os.Stdout.WriteString("\033[?1049h\033[?25l")
	defer os.Stdout.WriteString("\033[?25h\033[?1049l")

	selectedIndex := 0
	scrollOffset := 0
	filterText := ""
	isFiltering := false

	buf := make([]byte, 64)

	for {
		termWidth, termHeight, err := term.GetSize(fd)
		if err != nil || termWidth <= 0 || termHeight <= 0 {
			termWidth = 100
			termHeight = 25
		}

		filtered := filterSessions(sessions, filterText)
		if len(filtered) == 0 {
			selectedIndex = 0
			scrollOffset = 0
		} else {
			if selectedIndex >= len(filtered) {
				selectedIndex = len(filtered) - 1
			}
			if selectedIndex < 0 {
				selectedIndex = 0
			}
		}

		// Fixed lines:
		// Header box: 3 lines
		// Status / filter line: 2 lines
		// Table header: 1 line
		// Footer separator + help line: 2 lines
		// Total fixed: 8 lines
		visibleRows := termHeight - 8
		if visibleRows < 3 {
			visibleRows = 3
		}

		// Adjust scroll offset to keep selectedIndex in viewport
		if selectedIndex < scrollOffset {
			scrollOffset = selectedIndex
		} else if selectedIndex >= scrollOffset+visibleRows {
			scrollOffset = selectedIndex - visibleRows + 1
		}
		if scrollOffset < 0 {
			scrollOffset = 0
		}
		if maxOffset := len(filtered) - visibleRows; maxOffset >= 0 && scrollOffset > maxOffset {
			scrollOffset = maxOffset
		}

		renderScreenFlickerFree(filtered, selectedIndex, scrollOffset, visibleRows, filterText, isFiltering, termWidth, termHeight)

		n, err := os.Stdin.Read(buf)
		if err != nil {
			return "quit", nil, err
		}
		if n == 0 {
			continue
		}

		// Check for escape sequences
		if buf[0] == 27 {
			if n == 1 {
				// Single Esc key
				if isFiltering {
					isFiltering = false
					filterText = ""
				} else {
					return "quit", nil, nil
				}
				continue
			}

			// Arrow keys & special keys (ANSI / VT100 / xterm)
			if n >= 3 && (buf[1] == '[' || buf[1] == 'O') {
				switch buf[2] {
				case 'A': // Up Arrow
					if selectedIndex > 0 {
						selectedIndex--
					}
					continue
				case 'B': // Down Arrow
					if selectedIndex < len(filtered)-1 {
						selectedIndex++
					}
					continue
				case 'H': // Home
					selectedIndex = 0
					continue
				case 'F': // End
					if len(filtered) > 0 {
						selectedIndex = len(filtered) - 1
					}
					continue
				}

				if n >= 4 && buf[1] == '[' && buf[3] == '~' {
					switch buf[2] {
					case '5': // Page Up
						selectedIndex -= visibleRows
						if selectedIndex < 0 {
							selectedIndex = 0
						}
						continue
					case '6': // Page Down
						selectedIndex += visibleRows
						if selectedIndex >= len(filtered) {
							selectedIndex = max(0, len(filtered)-1)
						}
						continue
					case '1', '7': // Home
						selectedIndex = 0
						continue
					case '4', '8': // End
						if len(filtered) > 0 {
							selectedIndex = len(filtered) - 1
						}
						continue
					case '3': // Delete key
						if !isFiltering && len(filtered) > 0 {
							return "delete", &filtered[selectedIndex], nil
						}
					}
				}
			}
			continue
		}

		b := buf[0]

		if isFiltering {
			switch b {
			case 13: // Enter: finish typing filter
				isFiltering = false
			case 127, 8: // Backspace
				if len(filterText) > 0 {
					filterText = filterText[:len(filterText)-1]
					selectedIndex = 0
					scrollOffset = 0
				}
			case 3: // Ctrl+C
				return "quit", nil, nil
			default:
				if b >= 32 && b <= 126 {
					filterText += string(b)
					selectedIndex = 0
					scrollOffset = 0
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
		case 'g': // Top (vim style)
			selectedIndex = 0
		case 'G': // Bottom (vim style)
			if len(filtered) > 0 {
				selectedIndex = len(filtered) - 1
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
	}
}

func renderScreenFlickerFree(
	sessions []db.Session,
	selected int,
	offset int,
	visibleRows int,
	filter string,
	isFiltering bool,
	termWidth int,
	termHeight int,
) {
	var b bytes.Buffer

	// Move cursor to top-left (no clear screen = no flashing)
	b.WriteString("\033[H")

	// Header box
	headerTitle := "ANTIGRAVITY CLI SESSION HELPER"
	b.WriteString("\033[1;36m┏" + strings.Repeat("━", max(10, termWidth-2)) + "┓\033[0m\033[K\r\n")
	pad := max(0, (termWidth-2-len(headerTitle))/2)
	b.WriteString(fmt.Sprintf("\033[1;36m┃%s\033[1;37m%s\033[1;36m%s┃\033[0m\033[K\r\n",
		strings.Repeat(" ", pad),
		headerTitle,
		strings.Repeat(" ", max(0, termWidth-2-pad-len(headerTitle)))))
	b.WriteString("\033[1;36m┗" + strings.Repeat("━", max(10, termWidth-2)) + "┛\033[0m\033[K\r\n")

	// Filter / Status line
	if isFiltering {
		b.WriteString(fmt.Sprintf(" \033[1;33m[Search]:\033[0m %s\033[5m█\033[0m \033[2m(Enter to finish, Esc to clear)\033[0m\033[K\r\n\r\n", filter))
	} else if filter != "" {
		b.WriteString(fmt.Sprintf(" \033[1;33m[Filter]:\033[0m %s \033[2m(Press '/' to edit, Esc to clear)\033[0m\033[K\r\n\r\n", filter))
	} else {
		countStr := fmt.Sprintf("(%d saved sessions)", len(sessions))
		b.WriteString(fmt.Sprintf(" \033[2mSelect a session to resume with Antigravity CLI %s\033[0m\033[K\r\n\r\n", countStr))
	}

	// Calculate responsive column widths
	// Total available width minus padding (approx 4 chars left + spaces between cols)
	availWidth := termWidth - 16
	if availWidth < 40 {
		availWidth = 40
	}
	nameWidth := max(18, availWidth*38/100)
	pathWidth := max(20, availWidth*48/100)
	timeWidth := 12

	// Table header
	headerLine := fmt.Sprintf("   \033[1;4m%-4s %-*s %-*s %-*s\033[0m",
		"#",
		nameWidth, "SESSION NAME",
		pathWidth, "PROJECT WORKSPACE",
		timeWidth, "LAST ACTIVE")
	b.WriteString(headerLine + "\033[K\r\n")

	if len(sessions) == 0 {
		b.WriteString("\r\n    \033[33mNo sessions matching filter.\033[0m\033[K\r\n")
		// Fill remaining visible rows
		for r := 1; r < visibleRows; r++ {
			b.WriteString("\033[K\r\n")
		}
	} else {
		endIdx := min(offset+visibleRows, len(sessions))
		renderedRows := 0

		for i := offset; i < endIdx; i++ {
			s := sessions[i]
			renderedRows++

			cursor := "  "
			colorPrefix := ""
			colorSuffix := "\033[0m"

			if i == selected {
				cursor = "\033[1;32m❯ \033[0m"
				colorPrefix = "\033[1;32;48;5;236m"
				colorSuffix = "\033[0m"
			}

			name := truncateStr(s.Name, nameWidth)
			path := formatPath(s.ProjectPath, pathWidth)
			relTime := formatRelativeTime(s.UpdatedAt)

			rowStr := fmt.Sprintf("%s%s%-4d %-*s %-*s %-*s%s",
				cursor,
				colorPrefix,
				i+1,
				nameWidth, name,
				pathWidth, path,
				timeWidth, relTime,
				colorSuffix)

			b.WriteString(rowStr + "\033[K\r\n")
		}

		// Fill remaining empty rows in viewport so cursor doesn't jump
		for r := renderedRows; r < visibleRows; r++ {
			b.WriteString("\033[K\r\n")
		}
	}

	// Scroll position indicator
	scrollInfo := ""
	if len(sessions) > visibleRows {
		scrollInfo = fmt.Sprintf(" [%d-%d of %d] ", offset+1, min(offset+visibleRows, len(sessions)), len(sessions))
	}
	sepLen := max(0, termWidth-len(scrollInfo)-2)
	b.WriteString("\033[2m" + strings.Repeat("─", sepLen/2) + "\033[1;33m" + scrollInfo + "\033[0;2m" + strings.Repeat("─", (sepLen+1)/2) + "\033[0m\033[K\r\n")

	// Footer hotkeys
	b.WriteString(" \033[1m[Enter]\033[0m Open  \033[1m[↑/↓]\033[0m Nav  \033[1m[n]\033[0m New  \033[1m[d]\033[0m Delete  \033[1m[r]\033[0m Rename  \033[1m[/]\033[0m Search  \033[1m[q]\033[0m Quit\033[K")

	// Clear anything below footer
	b.WriteString("\033[J")

	if isFiltering {
		b.WriteString("\033[?25h") // Show cursor in filter mode
	} else {
		b.WriteString("\033[?25l") // Hide cursor in browse mode
	}

	// Write entire frame in a single atomic syscall
	_, _ = os.Stdout.Write(b.Bytes())
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
