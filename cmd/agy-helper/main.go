package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/antigravity"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/config"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/db"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/runner"
	"github.com/Sidharth-Prabhu/antigravity-session-helper/internal/ui"
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

	binName := strings.TrimSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".exe")
	args := os.Args[1:]

	// If invoked directly as 'agy' (e.g. via symlink, wrapper, or renamed binary)
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
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not determine user home directory: %v\n", err)
		return
	}

	if runtime.GOOS == "windows" {
		installWindowsPowerShellIntegration(home)
		return
	}

	installUnixShellIntegration(home)
}

func installWindowsPowerShellIntegration(home string) {
	psSnippet := `
# --- Antigravity CLI Helper Integration ---
function agy {
    if ($args.Count -eq 0) {
        & agy-session-helper.exe
    } else {
        & agy-session-helper.exe exec @args
    }
}
# ------------------------------------------
`
	candidates := []string{
		filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
		filepath.Join(home, "Documents", "WindowsPowerShell", "Microsoft.PowerShell_profile.ps1"),
	}

	installed := false
	for _, profilePath := range candidates {
		content, _ := os.ReadFile(profilePath)
		if strings.Contains(string(content), "Antigravity CLI Helper Integration") {
			fmt.Printf("PowerShell integration is already installed in %s!\n", profilePath)
			return
		}

		_ = os.MkdirAll(filepath.Dir(profilePath), 0755)
		f, err := os.OpenFile(profilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			_, _ = f.WriteString(psSnippet)
			_ = f.Close()
			fmt.Printf("✓ Successfully added PowerShell integration to %s!\n", profilePath)
			fmt.Println("Restart your PowerShell terminal or run: . $PROFILE")
			installed = true
			break
		}
	}

	if !installed {
		fmt.Println("Could not automatically locate PowerShell profile. Add this snippet to your $PROFILE:")
		fmt.Println(psSnippet)
	}
}

func installUnixShellIntegration(home string) {
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
	targets := []string{
		filepath.Join(home, ".zshrc"),
		filepath.Join(home, ".bashrc"),
	}

	installedCount := 0
	for _, target := range targets {
		content, err := os.ReadFile(target)
		if err != nil && !os.IsNotExist(err) {
			continue
		}
		if strings.Contains(string(content), "Antigravity CLI Helper Integration") {
			fmt.Printf("Shell integration is already installed in %s!\n", target)
			installedCount++
			continue
		}

		f, err := os.OpenFile(target, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			continue
		}
		if _, err := f.WriteString(snippet); err == nil {
			fmt.Printf("✓ Successfully added shell integration to %s!\n", target)
			installedCount++
		}
		_ = f.Close()
	}

	if installedCount > 0 {
		fmt.Println("Run: source ~/.zshrc (or source ~/.bashrc) to activate in current terminal.")
	} else {
		fmt.Println("Add this snippet to your shell profile (~/.zshrc, ~/.bashrc, or ~/.profile):")
		fmt.Println(snippet)
	}
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
