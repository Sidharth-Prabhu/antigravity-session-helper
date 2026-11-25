#!/usr/bin/env sh
# Antigravity CLI Session Helper (agy-session-helper) Universal Installer
# Compatible with macOS and Linux (x86_64, arm64)
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/Sidharth-Prabhu/antigravity-session-helper/main/install.sh | bash

set -e

REPO="Sidharth-Prabhu/antigravity-session-helper"
BINARY_NAME="agy-session-helper"
INSTALL_DIR="${HOME}/.local/bin"

# Colors for terminal output
BOLD="\033[1m"
GREEN="\033[32m"
CYAN="\033[36m"
YELLOW="\033[33m"
RED="\033[31m"
RESET="\033[0m"

printf "\n%b┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓%b\n" "$CYAN" "$RESET"
printf "%b┃           Antigravity CLI Session Helper Installer             ┃%b\n" "$CYAN" "$RESET"
printf "%b┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛%b\n\n" "$CYAN" "$RESET"

# 1. Detect OS
OS_RAW="$(uname -s)"
case "${OS_RAW}" in
    Darwin)
        OS="darwin"
        ;;
    Linux)
        OS="linux"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        OS="windows"
        ;;
    *)
        printf "%b[!] Unsupported Operating System: %s%b\n" "$RED" "${OS_RAW}" "$RESET"
        exit 1
        ;;
esac

# 2. Detect Architecture
ARCH_RAW="$(uname -m)"
case "${ARCH_RAW}" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        printf "%b[!] Unsupported CPU architecture: %s%b\n" "$RED" "${ARCH_RAW}" "$RESET"
        exit 1
        ;;
esac

printf "%b[*] Detected System:%b %s (%s)\n" "$BOLD" "$RESET" "${OS}" "${ARCH}"

# 3. Create install directory
mkdir -p "${INSTALL_DIR}"

TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'agy-helper')"
trap 'rm -rf "${TMP_DIR}"' EXIT

INSTALLED=0

# 4. Attempt downloading prebuilt release binary
TAR_NAME="antigravity-session-helper-${OS}-${ARCH}.tar.gz"
RELEASE_URL="https://github.com/${REPO}/releases/latest/download/${TAR_NAME}"

printf "%b[*] Checking for release binary...%b\n" "$BOLD" "$RESET"

if curl -sSLf "${RELEASE_URL}" -o "${TMP_DIR}/${TAR_NAME}" 2>/dev/null; then
    printf "%b[*] Extracting prebuilt binary...%b\n" "$BOLD" "$RESET"
    tar -xzf "${TMP_DIR}/${TAR_NAME}" -C "${TMP_DIR}"
    if [ -f "${TMP_DIR}/${BINARY_NAME}" ]; then
        mv "${TMP_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
        INSTALLED=1
    fi
fi

# 5. Fallback: Build from source using Go if release binary is not available
if [ "$INSTALLED" -ne 1 ]; then
    printf "%b[*] Release binary not found or unreachable. Checking for Go...%b\n" "$YELLOW" "$RESET"
    if command -v go >/dev/null 2>&1; then
        printf "%b[*] Compiling latest release with Go...%b\n" "$BOLD" "$RESET"
        if GOBIN="${INSTALL_DIR}" go install "github.com/${REPO}/cmd/agy-helper@latest"; then
            if [ -f "${INSTALL_DIR}/agy-helper" ]; then
                mv "${INSTALL_DIR}/agy-helper" "${INSTALL_DIR}/${BINARY_NAME}"
            fi
            chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
            INSTALLED=1
        fi
    fi
fi

# 6. Fallback: Clone repository and build
if [ "$INSTALLED" -ne 1 ]; then
    if command -v git >/dev/null 2>&1 && command -v go >/dev/null 2>&1; then
        printf "%b[*] Cloning repository to build from source...%b\n" "$BOLD" "$RESET"
        git clone --depth 1 "https://github.com/${REPO}.git" "${TMP_DIR}/src" >/dev/null 2>&1
        (cd "${TMP_DIR}/src" && go build -o "${INSTALL_DIR}/${BINARY_NAME}" ./cmd/agy-helper)
        chmod +x "${INSTALL_DIR}/${BINARY_NAME}"
        INSTALLED=1
    fi
fi

if [ "$INSTALLED" -ne 1 ]; then
    printf "\n%b[!] Installation failed.%b\n" "$RED" "$RESET"
    printf "Please ensure either curl can reach GitHub releases, or install Go (https://go.dev) and run:\n"
    printf "  go install github.com/%s/cmd/agy-helper@latest\n\n" "${REPO}"
    exit 1
fi

# 7. Create 'ash' symlink
ln -sf "${INSTALL_DIR}/${BINARY_NAME}" "${INSTALL_DIR}/ash"

printf "%b[✓] Binary installed to: %s/%s%b\n" "$GREEN" "${INSTALL_DIR}" "${BINARY_NAME}" "$RESET"
printf "%b[✓] Shortcut created:     %s/ash%b\n" "$GREEN" "${INSTALL_DIR}" "$RESET"

# 8. Check PATH
case ":$PATH:" in
    *":${INSTALL_DIR}:"*)
        ;;
    *)
        printf "%b[*] Adding %s to your PATH...%b\n" "$YELLOW" "${INSTALL_DIR}" "$RESET"
        for RC in "${HOME}/.zshrc" "${HOME}/.bashrc" "${HOME}/.profile"; do
            if [ -f "$RC" ]; then
                echo "export PATH=\"\$HOME/.local/bin:\$PATH\"" >> "$RC"
            fi
        done
        export PATH="${INSTALL_DIR}:${PATH}"
        ;;
esac

# 9. Configure Shell Integration
printf "%b[*] Configuring shell integration...%b\n" "$BOLD" "$RESET"
"${INSTALL_DIR}/${BINARY_NAME}" setup-shell

printf "\n%b━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%b\n" "$GREEN" "$RESET"
printf "%b✨ Antigravity Session Helper installed successfully!%b\n" "$GREEN" "$RESET"
printf "%b━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%b\n\n" "$GREEN" "$RESET"
printf "To start using it right away:\n"
printf "  1. Reload your shell:  %bsource ~/.zshrc%b (or %bsource ~/.bashrc%b)\n" "$BOLD" "$RESET" "$BOLD" "$RESET"
printf "  2. Launch browser:     %bash%b (or %bagy%b)\n" "$CYAN" "$RESET" "$CYAN" "$RESET"
printf "  3. Start a session:    %bagy --new-project%b\n\n" "$CYAN" "$RESET"
