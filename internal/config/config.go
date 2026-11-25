package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type Config struct {
	ConfigDir     string
	DBPath        string
	AgyAppDataDir string
	AgyBinaryPath string
}

func LoadConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := os.Getenv("AGY_HELPER_CONFIG_DIR")
	if configDir == "" {
		userConfig, err := os.UserConfigDir()
		if err == nil {
			configDir = filepath.Join(userConfig, "agy-session-helper")
		} else {
			configDir = filepath.Join(homeDir, ".config", "agy-session-helper")
		}
	}

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	dbPath := os.Getenv("AGY_HELPER_DB")
	if dbPath == "" {
		dbPath = filepath.Join(configDir, "sessions.db")
	}

	agyDataDir := os.Getenv("ANTIGRAVITY_APP_DATA_DIR")
	if agyDataDir == "" {
		agyDataDir = filepath.Join(homeDir, ".gemini", "antigravity-cli")
	}

	agyBin := findRealAgyBinary()

	return &Config{
		ConfigDir:     configDir,
		DBPath:        dbPath,
		AgyAppDataDir: agyDataDir,
		AgyBinaryPath: agyBin,
	}, nil
}

// findRealAgyBinary finds the genuine agy binary on the system, avoiding self-referencing wrappers.
func findRealAgyBinary() string {
	selfPath, _ := os.Executable()
	selfPath, _ = filepath.EvalSymlinks(selfPath)

	homeDir, _ := os.UserHomeDir()
	isWindows := runtime.GOOS == "windows"

	var exeNames []string
	if isWindows {
		exeNames = []string{"agy.exe", "agy.cmd", "agy.bat", "agy"}
	} else {
		exeNames = []string{"agy"}
	}

	// Common candidate directories across Windows, macOS, Linux
	candidateDirs := []string{
		filepath.Join(homeDir, ".local", "bin"),
		"/usr/local/bin",
		"/usr/bin",
		"/opt/homebrew/bin",
	}

	if isWindows {
		if localApp := os.Getenv("LOCALAPPDATA"); localApp != "" {
			candidateDirs = append(candidateDirs,
				filepath.Join(localApp, "Programs", "antigravity-cli", "bin"),
				filepath.Join(localApp, "Programs", "agy"),
				filepath.Join(localApp, "Microsoft", "WindowsApps"),
			)
		}
		if progFiles := os.Getenv("ProgramFiles"); progFiles != "" {
			candidateDirs = append(candidateDirs, filepath.Join(progFiles, "Antigravity"))
		}
		if progFilesX86 := os.Getenv("ProgramFiles(x86)"); progFilesX86 != "" {
			candidateDirs = append(candidateDirs, filepath.Join(progFilesX86, "Antigravity"))
		}
	}

	for _, dir := range candidateDirs {
		for _, name := range exeNames {
			cand := filepath.Join(dir, name)
			if cand == selfPath {
				continue
			}
			if isExecutableFile(cand) {
				return cand
			}
		}
	}

	// Search PATH
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		for _, name := range exeNames {
			cand := filepath.Join(dir, name)
			if cand == selfPath {
				continue
			}
			evalCand, err := filepath.EvalSymlinks(cand)
			if err == nil && evalCand == selfPath {
				continue
			}
			if isExecutableFile(cand) {
				return cand
			}
		}
	}

	// Fallback to exec.LookPath
	for _, name := range exeNames {
		if p, err := exec.LookPath(name); err == nil {
			evalP, _ := filepath.EvalSymlinks(p)
			if evalP != selfPath {
				return p
			}
		}
	}

	if isWindows {
		return "agy.exe"
	}
	return "agy"
}

func isExecutableFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		return false
	}
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".cmd" || ext == ".bat"
	}
	return fi.Mode()&0111 != 0
}
