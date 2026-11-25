package config

import (
	"os"
	"os/exec"
	"path/filepath"
)

type Config struct {
	ConfigDir       string
	DBPath          string
	AgyAppDataDir   string
	AgyBinaryPath   string
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

	// Check common installation locations first
	homeDir, _ := os.UserHomeDir()
	candidatePaths := []string{
		filepath.Join(homeDir, ".local", "bin", "agy"),
		"/usr/local/bin/agy",
		"/opt/homebrew/bin/agy",
	}

	for _, cand := range candidatePaths {
		if cand == selfPath {
			continue
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
			return cand
		}
	}

	// Otherwise search PATH
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		cand := filepath.Join(dir, "agy")
		if cand == selfPath {
			continue
		}
		evalCand, err := filepath.EvalSymlinks(cand)
		if err == nil && evalCand == selfPath {
			continue
		}
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() && (fi.Mode()&0111 != 0) {
			return cand
		}
	}

	// Fallback to exec.LookPath
	if p, err := exec.LookPath("agy"); err == nil {
		return p
	}

	return "agy"
}
