package runner

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/sidharth/agy-session-helper/internal/antigravity"
	"github.com/sidharth/agy-session-helper/internal/config"
	"github.com/sidharth/agy-session-helper/internal/db"
)

// NonInteractiveSubcommands are agy subcommands that don't produce persistent interactive chat sessions.
var nonInteractiveSubcommands = map[string]bool{
	"help":           true,
	"changelog":      true,
	"update":         true,
	"models":         true,
	"mcp":            true,
	"plugin":         true,
	"plugins":        true,
	"agent":          true,
	"agents":         true,
	"mic-serve":      true,
	"remote-control": true,
	"acp":            true,
	"install":        true,
}

// IsInteractiveSession determines if the CLI invocation is an interactive conversation session.
func IsInteractiveSession(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "-v" || arg == "--version" {
			return false
		}
		if arg == "-p" || arg == "--print" || arg == "--prompt" {
			// Print mode runs non-interactively
			return false
		}
		if nonInteractiveSubcommands[arg] {
			return false
		}
	}
	return true
}

// RunAgy executes the real agy binary, passing terminal I/O directly.
// When the session ends, it prompts the user to save or delete the session.
func RunAgy(cfg *config.Config, store *db.Store, args []string, workingDir string) error {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			workingDir = "."
		}
	}
	workingDir, _ = filepath.Abs(workingDir)

	// If not an interactive session, simply passthrough and exit
	if !IsInteractiveSession(args) {
		cmd := exec.Command(cfg.AgyBinaryPath, args...)
		cmd.Dir = workingDir
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	startTime := time.Now()

	// Check if conversation ID was already specified in flags
	explicitConvoID := antigravity.ExtractFromArgs(args)

	cmd := exec.Command(cfg.AgyBinaryPath, args...)
	cmd.Dir = workingDir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Forward signals to child process
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start antigravity CLI (%s): %w", cfg.AgyBinaryPath, err)
	}

	go func() {
		for sig := range sigChan {
			if cmd.Process != nil {
				_ = cmd.Process.Signal(sig)
			}
		}
	}()

	// Wait for agy to finish
	cmdErr := cmd.Wait()

	// Post-session handling: find conversation ID
	var convoID string
	var suggestedTitle string

	if explicitConvoID != "" {
		convoID = explicitConvoID
	}

	// Wait a moment for disk flushes (e.g. SQLite WAL checkpointing in agy)
	time.Sleep(200 * time.Millisecond)

	if convoID == "" {
		detected, err := antigravity.DetectRecentConversation(cfg.AgyAppDataDir, workingDir, startTime)
		if err == nil && detected != nil {
			convoID = detected.ConversationID
			suggestedTitle = detected.Title
		}
	} else if detected, err := antigravity.DetectRecentConversation(cfg.AgyAppDataDir, workingDir, startTime); err == nil && detected != nil {
		suggestedTitle = detected.Title
	}

	// If still no conversation ID found, we cannot prompt for session saving
	if convoID == "" {
		return cmdErr
	}

	// Interactive prompt to save or delete
	promptSaveOrDelete(cfg, store, convoID, suggestedTitle, workingDir)

	return cmdErr
}

func promptSaveOrDelete(cfg *config.Config, store *db.Store, convoID, suggestedTitle, workingDir string) {
	reader := bufio.NewReader(os.Stdin)

	// Check if already in store
	existingSess, _ := store.GetSessionByConversationID(convoID)

	fmt.Println()
	fmt.Println("\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")
	fmt.Println("\033[1;32m  ✨ Antigravity CLI Session Finished\033[0m")
	fmt.Printf("  \033[1mSession ID:\033[0m   %s\n", convoID)
	fmt.Printf("  \033[1mProject Path:\033[0m %s\n", workingDir)
	if existingSess != nil {
		fmt.Printf("  \033[1mSaved As:\033[0m     %s\n", existingSess.Name)
	} else if suggestedTitle != "" {
		fmt.Printf("  \033[1mTopic:\033[0m        %s\n", truncate(suggestedTitle, 60))
	}
	fmt.Println("\033[1;36m━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\033[0m")

	if existingSess != nil {
		fmt.Printf("\033[1mThis session is already saved as '%s'. Update timestamp? [Y/n/rename/delete]: \033[0m", existingSess.Name)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))

		if resp == "d" || resp == "delete" {
			confirmDelete(cfg, store, convoID, reader)
			return
		} else if resp == "r" || resp == "rename" {
			fmt.Print("\033[1mEnter new session name: \033[0m")
			newName, _ := reader.ReadString('\n')
			newName = strings.TrimSpace(newName)
			if newName != "" {
				_ = store.RenameSession(convoID, newName)
				fmt.Printf("\033[1;32m✓ Renamed session to '%s'\033[0m\n", newName)
			}
			return
		} else if resp == "n" || resp == "no" {
			return
		} else {
			// Update timestamp
			_, _ = store.SaveSession(existingSess.Name, convoID, workingDir, existingSess.Notes)
			fmt.Println("\033[1;32m✓ Session timestamp updated.\033[0m")
			return
		}
	}

	// New session: prompt save or delete
	for {
		fmt.Print("\033[1mSave or delete this session? (y/n): \033[0m")
		input, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		ans := strings.TrimSpace(strings.ToLower(input))

		if ans == "y" || ans == "yes" {
			// Prompt for name
			defaultName := suggestedTitle
			if defaultName == "" {
				defaultName = fmt.Sprintf("Session %s", convoID[:8])
			}

			fmt.Printf("\033[1mEnter a name for this session\033[0m [%s]: ", defaultName)
			nameInput, _ := reader.ReadString('\n')
			name := strings.TrimSpace(nameInput)
			if name == "" {
				name = defaultName
			}

			sess, err := store.SaveSession(name, convoID, workingDir, "")
			if err != nil {
				fmt.Printf("\033[1;31m✗ Error saving session: %v\033[0m\n", err)
			} else {
				fmt.Println()
				fmt.Printf("\033[1;32m✓ Session '%s' saved to database!\033[0m\n", sess.Name)
				fmt.Printf("  Resume anytime: \033[33magy --conversation=%s\033[0m or use this helper\n", convoID)
			}
			break
		} else if ans == "n" || ans == "no" {
			confirmDelete(cfg, store, convoID, reader)
			break
		} else {
			fmt.Println("\033[33mPlease enter 'y' to save or 'n' to delete/discard.\033[0m")
		}
	}
}

func confirmDelete(cfg *config.Config, store *db.Store, convoID string, reader *bufio.Reader) {
	fmt.Print("\033[1;31mPermanently delete this session and its files from Antigravity? (y/n): \033[0m")
	input, _ := reader.ReadString('\n')
	ans := strings.TrimSpace(strings.ToLower(input))

	if ans == "y" || ans == "yes" {
		errAgy := antigravity.DeleteAgySession(cfg.AgyAppDataDir, convoID)
		errStore := store.DeleteSession(convoID)
		if errAgy != nil {
			fmt.Printf("\033[1;33m⚠ Some files could not be removed: %v\033[0m\n", errAgy)
		} else {
			fmt.Println("\033[1;32m✓ Session permanently deleted from Antigravity and helper.\033[0m")
		}
		_ = errStore
	} else {
		fmt.Println("\033[2mSession discarded without saving.\033[0m")
	}
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}
