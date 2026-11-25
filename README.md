<div align="center">

# 🚀 Antigravity CLI Session Helper (`ash`)

**A robust, zero-glitch session supervisor and interactive session manager for Google Antigravity CLI (`agy`).**

[![Go Version](https://img.shields.io/github/go-mod/go-version/Sidharth-Prabhu/antigravity-session-helper?style=flat-square)](https://go.dev)
[![Platform Support](https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue?style=flat-square)](https://github.com/Sidharth-Prabhu/antigravity-session-helper)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)

Never lose or manually copy an `agy --conversation=<uuid>` ID again.

</div>

---

## ⚡ Quick Install (1-Line Command)

### 🍎 macOS & 🐧 Linux

Run this one-liner in your terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.sh | bash
```

*Or if you prefer `wget`:*
```bash
wget -qO- https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.sh | bash
```

---

### 🪟 Windows (PowerShell)

Open PowerShell and run:

```powershell
irm https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.ps1 | iex
```

*Or via CMD:*
```cmd
curl.exe -fsSL https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.ps1 | powershell -Command -
```

---

### 🐹 With Go (Any Platform)

```bash
go install github.com/Sidharth-Prabhu/antigravity-session-helper/cmd/agy-helper@latest
```

---

## 🎯 The Problem & The Solution

### The Problem
When using Google Antigravity CLI (`agy`), conversations terminate with a prompt like:
```bash
To continue this conversation, run: agy --conversation=0fa98ade-3060-4546-ada4-0652329fdfb6
```
Remembering, copying, or bookmarking random 36-character UUIDs across different project repositories quickly becomes chaotic.

### The Solution
1. **Background Supervisor**: Whenever `agy` is launched, the helper silently runs in the background. It forwards all terminal I/O, raw ANSI modes, colors, and OS signals (`SIGINT`, `SIGTERM`) without any input lag or display interference.
2. **Session Completion Prompt**: The instant your `agy` session ends, the helper detects the conversation ID and prompts you:
   ```text
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
     ✨ Antigravity CLI Session Finished
     Session ID:   0fa98ade-3060-4546-ada4-0652329fdfb6
     Project Path: /Users/username/development/my-project
   ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
   Save or delete this session? (y/n): 
   ```
   - **Press `y`**: Prompts you for a friendly name (suggesting the auto-generated conversation topic) and saves it to a local SQLite database.
   - **Press `n`**: Offers to permanently purge the conversation and temporary brain artifacts from Antigravity so they don't clutter your disk.
3. **Interactive Terminal Browser (TUI)**: Run `agy` or `ash` anytime with no arguments to open a flicker-free interactive session browser. Select any previous session with the arrow keys, hit `Enter`, and it automatically changes to that project's directory and resumes your session in `agy`!

---

## 🖥️ Interactive Session Browser

Running `agy-session-helper` (or `ash`) opens the interactive viewport:

```text
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃                         ANTIGRAVITY CLI SESSION HELPER                           ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
 Select a session to resume with Antigravity CLI (50 saved sessions)

   #    SESSION NAME                   PROJECT WORKSPACE              LAST ACTIVE
❯  1    CAT Marks API Fix              ~/development/RIT-Backend      25m ago
   2    Website Security Assessment    ~/development/web-audit        2h ago
   3    Windows ISO Flashing Tool      ~/development/usb-tool         1d ago
   4    Raspberry Pi Monitor Setup     ~/projects/rpi-server          3d ago
   5    Mac Cursor Glitch Fix          ~/development/crossover        Nov 21
──────────────────────────────────────────────────────────────────────────────────
 [Enter] Open  [↑/↓] Nav  [n] New  [d] Delete  [r] Rename  [/] Search  [q] Quit
```

### ⌨️ Keyboard Controls

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `Page Up` / `Page Down` | Scroll viewport by page |
| `Home` / `End` (or `g` / `G`) | Jump to top / bottom |
| `Enter` | **Resume session** (automatically `cd`s to project path and launches `agy --conversation=<id>`) |
| `/` | **Live Search**: Filter sessions by name, workspace path, or session UUID |
| `n` | **New Session**: Launch a fresh `agy` session in current directory |
| `d` | **Delete**: Delete session from helper database (with prompt to also wipe Antigravity files) |
| `r` | **Rename**: Rename session label |
| `i` | **Import History**: Scan and import unmanaged sessions from Antigravity CLI history |
| `q` / `Esc` / `Ctrl+C` | Exit browser |

---

## 🛠️ CLI Commands

```bash
# Open interactive session browser
ash
# or
agy-session-helper

# Launch an agy session with supervisor & auto-save prompt on exit
agy-session-helper exec [args...]

# List all saved sessions in a clean tabular view
ash list

# Resume a specific session directly by friendly name or ID
ash open "CAT Marks API"
ash open 0fa98ade-3060-4546-ada4-0652329fdfb6

# Delete a session
ash delete 0fa98ade-3060-4546-ada4-0652329fdfb6

# Import existing historical sessions from Antigravity CLI
ash import

# Configure shell integration for your current shell
ash setup-shell
```

---

## 🐚 Shell Integration

The installer automatically configures shell integration so you can use `agy` naturally:

### Zsh (`~/.zshrc`) / Bash (`~/.bashrc`)
```bash
agy() {
    if [ "$#" -eq 0 ]; then
        agy-session-helper
    else
        agy-session-helper exec "$@"
    fi
}
```

### Windows PowerShell (`$PROFILE`)
```powershell
function agy {
    if ($args.Count -eq 0) {
        & agy-session-helper.exe
    } else {
        & agy-session-helper.exe exec @args
    }
}
```

- Type `agy` with **no arguments** ➔ Opens the Interactive Session Browser.
- Type `agy <prompt/flags>` ➔ Starts `agy` with background tracking and prompts to save or delete upon exit.

---

## 🏛️ Architecture & Robustness

- **Zero-Flicker TUI**: Uses the terminal **alternate screen buffer** (`\033[?1049h`), single-syscall buffer flushes, and in-place row updates (`\033[H` + `\033[K`) with viewport windowing. Never flashes or scrolls off-screen even with hundreds of sessions.
- **Pure Go CGO-Free SQLite (`modernc.org/sqlite`)**: No external C compiler, GCC, or dynamic libraries needed. Fully cross-compiles to Linux, macOS, and Windows.
- **Resilient Multi-Tier Conversation Detection**:
  1. CLI argument parser (`--conversation`, `-r`, `--resume`).
  2. Workspace correlation in `conversation_summaries.db`.
  3. `history.jsonl` workspace correlation.
  4. Regex extraction from `cli.log` (`Created conversation <uuid>`, `Streaming conversation <uuid>`).
  5. File modification timestamps in `~/.gemini/antigravity-cli/conversations/`.
- **Thorough Session Deletion**: When you choose to delete a session, it purges:
  - The record in the SQLite sessions database.
  - The conversation database files: `<appDataDir>/conversations/<id>.db*`.
  - The brain scratch directory: `<appDataDir>/brain/<id>`.
  - Full-text search and metadata entries in `conversation_summaries.db`, `conversations_fts`, and `conversation_parents`.

---

## 📂 Data Storage Locations

| OS | Sessions Database Path | Antigravity CLI Data Path |
|---|---|---|
| **macOS** | `~/Library/Application Support/agy-session-helper/sessions.db` | `~/.gemini/antigravity-cli` |
| **Linux** | `~/.config/agy-session-helper/sessions.db` | `~/.gemini/antigravity-cli` |
| **Windows** | `%APPDATA%\agy-session-helper\sessions.db` | `%USERPROFILE%\.gemini\antigravity-cli` |

*Override database path anytime with the `AGY_HELPER_DB` environment variable.*

---

## 📦 Supported Platforms & Architectures

| OS | Architectures | Prebuilt Binaries |
|---|---|---|
| **macOS** | Apple Silicon (`arm64`), Intel (`amd64`) | `antigravity-session-helper-darwin-*.tar.gz` |
| **Linux** | 64-bit (`amd64`), ARM64 / Raspberry Pi (`arm64`) | `antigravity-session-helper-linux-*.tar.gz` |
| **Windows** | 64-bit (`amd64`), ARM64 (`arm64`) | `antigravity-session-helper-windows-*.zip` |

---

## 📄 License

MIT License. Developed with ❤️ for Antigravity CLI users.
