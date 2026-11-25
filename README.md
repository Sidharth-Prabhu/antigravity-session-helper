# Antigravity CLI Session Helper (`agy-session-helper`)

A robust, lightweight Go utility designed to streamline and manage your **Google Antigravity CLI (`agy`)** sessions.

---

## 🌟 Why This Exists

When working with `agy` in your terminal:
- At the end of every conversation, `agy` prints out a resume command with a UUID session ID:
  ```bash
  agy --conversation=0fa98ade-3060-4546-ada4-0652329fdfb6
  ```
- Remembering, copying, and organizing these UUIDs across multiple projects is tedious.
- **`agy-session-helper`** runs silently in the background while `agy` is running.
- When your session finishes:
  1. It prompts you: **"Save or delete this session? (y/n)"**
  2. If you press **`y`**, it prompts for a human-readable name and saves it to a local SQLite database along with the project workspace and timestamp.
  3. If you press **`n`**, it gives you the option to permanently purge the conversation and its temporary artifacts.
  4. Running the helper opens an interactive, searchable terminal UI displaying all your saved sessions and workspaces. Selecting one automatically changes to the project directory and resumes the conversation in `agy`.

---

## 🚀 Quick Start

### 1. Build and Install

```bash
# Build binary
go build -o agy-session-helper ./cmd/agy-helper

# Install to ~/.local/bin (or your preferred PATH directory)
cp agy-session-helper ~/.local/bin/agy-session-helper
ln -sf ~/.local/bin/agy-session-helper ~/.local/bin/ash
```

### 2. Enable Seamless Shell Integration

Run the built-in installer:
```bash
agy-session-helper setup-shell
source ~/.zshrc
```

This adds the following wrapper to your shell profile:
```zsh
agy() {
    if [ "$#" -eq 0 ]; then
        agy-session-helper
    else
        agy-session-helper exec "$@"
    fi
}
```

Now:
- Typing `agy` with no arguments opens the **Interactive Session Browser**.
- Typing `agy <flags/prompt>` runs the CLI with background session tracking and automatic save/delete prompting when done.

---

## 🛠️ Usage & Commands

| Command | Description |
|---|---|
| `agy-session-helper` (or `ash`) | Opens the interactive terminal browser to view, search, and resume saved sessions. |
| `agy-session-helper exec [args...]` | Runs `agy` in the foreground with native terminal passthrough while supervising the session in the background. |
| `agy-session-helper list` | Prints a clean tabular list of all saved sessions. |
| `agy-session-helper open <id\|name>` | Quickly resumes a specific session by conversation ID or friendly name. |
| `agy-session-helper delete <id>` | Deletes a session from the database and purges its Antigravity conversation files. |
| `agy-session-helper import` | Automatically imports existing conversations from Antigravity history into the helper database. |
| `agy-session-helper setup-shell` | Appends the `agy()` wrapper function to your `~/.zshrc`. |

---

## 🎮 Interactive Browser Navigation

When running `agy-session-helper` without arguments:

| Key | Action |
|---|---|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `Enter` | Open selected session in `agy` (navigates to workspace automatically) |
| `/` | Live search / filter sessions by name, workspace, or session ID |
| `n` | Launch a brand new `agy` session in current directory |
| `d` | Delete selected session (prompts to also delete Antigravity files) |
| `r` | Rename selected session |
| `i` | Scan and import past Antigravity conversations |
| `q` / `Esc` / `Ctrl+C` | Quit browser |

---

## 🏗️ Architecture & Robustness

- **Zero Terminal Distortion**: Uses native stdio file descriptors (`os.Stdin`, `os.Stdout`, `os.Stderr`) and signal multiplexing (`SIGINT`, `SIGTERM`, `SIGHUP`) during `agy` execution so that full-screen raw modes, mouse events, and ANSI formatting are 100% authentic.
- **Multi-layered Session Detection**:
  1. CLI arguments inspection (`--conversation`, `-r`).
  2. `conversation_summaries.db` inspection for workspace match and latest timestamp.
  3. `history.jsonl` workspace correlation.
  4. `cli.log` regex extraction (`Created conversation <uuid>`, `Streaming conversation <uuid>`).
  5. `conversations/*.db` filesystem timestamp analysis.
- **Self-contained SQLite Storage**:
  - Uses `modernc.org/sqlite` (pure Go, CGO-free, cross-platform).
  - WAL journal mode enabled with auto-retry for crash resilience and concurrency.
- **Clean Deletion**: When deleting a conversation, purges:
  - SQLite database entry.
  - `~/.gemini/antigravity-cli/conversations/<id>.db*`.
  - `~/.gemini/antigravity-cli/brain/<id>`.
  - Records in `conversation_summaries.db`, `conversations_fts`, and `conversation_parents`.
