package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var loadEnvOnce sync.Once

func GetString(key string, defaultValue string) string {
	ensureEnvLoaded()

	value, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return defaultValue
	}

	return value
}

func MustString(key string) (string, error) {
	ensureEnvLoaded()

	value, ok := os.LookupEnv(key)
	if !ok {
		return "", fmt.Errorf("required environment variable %q is not set", key)
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("required environment variable %q is empty", key)
	}

	return value, nil
}

func GetInt(key string, defaultValue int) (int, error) {
	ensureEnvLoaded()

	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("parse %q as int: %w", key, err)
	}

	return parsed, nil
}

func GetBool(key string, defaultValue bool) (bool, error) {
	ensureEnvLoaded()

	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("parse %q as bool: %w", key, err)
	}

	return parsed, nil
}

func GetDuration(key string, defaultValue time.Duration) (time.Duration, error) {
	ensureEnvLoaded()

	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("parse %q as duration: %w", key, err)
	}

	return parsed, nil
}

func ensureEnvLoaded() {
	loadEnvOnce.Do(func() {
		dir, err := os.Getwd()
		if err != nil {
			return
		}

		envPaths := findEnvFiles(dir)
		for _, path := range envPaths {
			_ = loadEnvFile(path)
		}
	})
}

func findEnvFiles(startDir string) []string {
	var envPath string
	var envLocalPath string

	dir := startDir
	for {
		candidateEnv := filepath.Join(dir, ".env")
		if fileExists(candidateEnv) {
			envPath = candidateEnv
		}

		candidateEnvLocal := filepath.Join(dir, ".env.local")
		if fileExists(candidateEnvLocal) {
			envLocalPath = candidateEnvLocal
		}

		if envPath != "" || envLocalPath != "" {
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	paths := make([]string, 0, 2)
	if envPath != "" {
		paths = append(paths, envPath)
	}
	if envLocalPath != "" {
		paths = append(paths, envLocalPath)
	}

	return paths
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)

		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		_ = os.Setenv(key, value)
	}

	return scanner.Err()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
