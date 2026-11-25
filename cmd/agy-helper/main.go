package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sidharth/agy-session-helper/internal/antigravity"
	"github.com/sidharth/agy-session-helper/internal/config"
	"github.com/sidharth/agy-session-helper/internal/db"
	"github.com/sidharth/agy-session-helper/internal/runner"
	"github.com/sidharth/agy-session-helper/internal/ui"
)

const version = "1.0.0"

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	store, err := db.InitDB(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	binName := filepath.Base(os.Args[0])
	args := os.Args[1:]

	// If invoked directly as 'agy' (e.g. via symlink or alias)
	if binName == "agy" {
		if len(args) == 0 {
			// Without arguments, launch interactive browser
			if err := ui.StartInteractiveBrowser(cfg, store); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		// With arguments, run agy session with supervisor
		if err := runner.RunAgy(cfg, store, args, ""); err != nil {
			if exitErr, ok := err.(*os.SyscallError); ok {
				os.Exit(1)
				_ = exitErr
			}
		}
		return
	}

	// If no arguments passed to agy-session-helper, start interactive browser
	if len(args) == 0 {
		if err := ui.StartInteractiveBrowser(cfg, store); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	switch args[0] {
	case "run", "exec", "launch":
		runArgs := args[1:]
		if err := runner.RunAgy(cfg, store, runArgs, ""); err != nil {
			os.Exit(1)
		}

	case "list", "ls":
		listSessions(store)

	case "open", "resume":
		if len(args) < 2 {
			fmt.Println("Usage: agy-session-helper open <session-id|name>")
			os.Exit(1)
		}
		openSession(cfg, store, args[1])

	case "delete", "rm":
		if len(args) < 2 {
			fmt.Println("Usage: agy-session-helper delete <conversation-id>")
			os.Exit(1)
		}
		deleteSession(cfg, store, args[1])

	case "import":
		importSessions(cfg, store)

	case "setup-shell", "install":
		installShellIntegration()

	case "version", "--version", "-v":
		fmt.Printf("agy-session-helper v%s\n", version)

	case "help", "--help", "-h":
		printHelp()

	default:
		// If unknown subcommand starts with '-', treat as flags passed to agy session runner
		if strings.HasPrefix(args[0], "-") {
			if err := runner.RunAgy(cfg, store, args, ""); err != nil {
				os.Exit(1)
			}
			return
		}

		fmt.Printf("Unknown command '%s'. Run 'agy-session-helper help' for usage.\n", args[0])
		os.Exit(1)
	}
}

func listSessions(store *db.Store) {
	sessions, err := store.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions. Launch an agy session to save one, or run 'agy-session-helper import'.")
		return
	}

	fmt.Println()
	fmt.Printf("%-3s  %-30s  %-35s  %-36s  %s\n", "#", "SESSION NAME", "PROJECT WORKSPACE", "CONVERSATION ID", "UPDATED")
	fmt.Println(strings.Repeat("-", 125))

	for i, s := range sessions {
		name := truncate(s.Name, 30)
		proj := truncate(s.ProjectPath, 35)
		fmt.Printf("%-3d  %-30s  %-35s  %-36s  %s\n",
			i+1, name, proj, s.ConversationID, s.UpdatedAt.Format("2006-01-02 15:04"))
	}
	fmt.Println()
}

func openSession(cfg *config.Config, store *db.Store, target string) {
	// Try lookup by ID
	sess, err := store.GetSessionByConversationID(target)
	if err != nil || sess == nil {
		// Try search by name
		sessions, _ := store.ListSessions()
		for _, s := range sessions {
			if strings.EqualFold(s.Name, target) || strings.HasPrefix(s.ConversationID, target) {
				sess = &s
				break
			}
		}
	}

	if sess == nil {
		fmt.Printf("Error: No session found matching '%s'\n", target)
		os.Exit(1)
	}

	targetDir := sess.ProjectPath
	if fi, err := os.Stat(targetDir); err != nil || !fi.IsDir() {
		targetDir, _ = os.Getwd()
	}

	fmt.Printf("🚀 Resuming session '%s' in %s...\n", sess.Name, targetDir)
	args := []string{"--conversation=" + sess.ConversationID}
	if err := runner.RunAgy(cfg, store, args, targetDir); err != nil {
		os.Exit(1)
	}
}

func deleteSession(cfg *config.Config, store *db.Store, target string) {
	sess, err := store.GetSessionByConversationID(target)
	if err != nil || sess == nil {
		fmt.Printf("Error: Session with ID '%s' not found.\n", target)
		os.Exit(1)
	}

	_ = store.DeleteSession(sess.ConversationID)
	_ = antigravity.DeleteAgySession(cfg.AgyAppDataDir, sess.ConversationID)
	fmt.Printf("✓ Deleted session '%s' (%s)\n", sess.Name, sess.ConversationID)
}

func importSessions(cfg *config.Config, store *db.Store) {
	convos, err := antigravity.ListAgyRawConversations(cfg.AgyAppDataDir, 50)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading Antigravity conversations: %v\n", err)
		os.Exit(1)
	}

	count := 0
	for _, c := range convos {
		existing, _ := store.GetSessionByConversationID(c.ConversationID)
		if existing == nil {
			title := c.Title
			if title == "" {
				title = fmt.Sprintf("Session %s", c.ConversationID[:8])
			}
			proj := c.ProjectPath
			if proj == "" {
				proj, _ = os.Getwd()
			}
			_, _ = store.SaveSession(title, c.ConversationID, proj, "Imported")
			count++
		}
	}

	fmt.Printf("✓ Successfully imported %d sessions from Antigravity history!\n", count)
}

func installShellIntegration() {
	home, _ := os.UserHomeDir()
	zshrc := filepath.Join(home, ".zshrc")

	snippet := `
# --- Antigravity CLI Helper Integration ---
agy() {
    if [ "$#" -eq 0 ]; then
        agy-session-helper
    else
        agy-session-helper exec "$@"
    fi
}
# ------------------------------------------
`

	// Check if already in ~/.zshrc
	content, _ := os.ReadFile(zshrc)
	if strings.Contains(string(content), "Antigravity CLI Helper Integration") {
		fmt.Println("Shell integration is already installed in ~/.zshrc!")
		return
	}

	f, err := os.OpenFile(zshrc, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Could not open %s for writing: %v\n", zshrc, err)
		fmt.Println("You can manually add this snippet to your shell configuration file:")
		fmt.Println(snippet)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(snippet); err != nil {
		fmt.Printf("Error writing to %s: %v\n", zshrc, err)
		return
	}

	fmt.Printf("✓ Successfully added shell integration to %s!\n", zshrc)
	fmt.Println("Run: source ~/.zshrc (or restart your terminal) to activate.")
}

func printHelp() {
	fmt.Println(`Antigravity CLI Session Helper (agy-session-helper)

A robust utility to manage, persist, and quickly resume Antigravity CLI (agy) sessions.

USAGE:
  agy-session-helper                     Open interactive session selector
  agy-session-helper exec [args...]      Run an agy session in foreground with background tracking
  agy-session-helper list                List all saved sessions
  agy-session-helper open <id|name>      Resume a session by ID or name
  agy-session-helper delete <id>         Delete a session by ID
  agy-session-helper import              Import recent conversations from Antigravity history
  agy-session-helper setup-shell         Install shell wrapper into ~/.zshrc
  agy-session-helper version             Show version

HOW IT WORKS:
  When launched with 'exec' (or through the 'agy' shell wrapper):
  1. It launches 'agy' in your terminal with complete native terminal support.
  2. The helper tracks the session in the background.
  3. When the session exits, the helper automatically detects the conversation ID
     and prompts you:
     - Save or delete this session? (y/n)
     - If 'y', prompts for a session name and stores it in SQLite.
     - If 'n', offers to permanently delete the session from Antigravity.
  4. Run 'agy-session-helper' anytime to see all saved sessions with project paths
     and press Enter to instantly resume!`)
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
