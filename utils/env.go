package utils

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func init() {
	LoadEnvFile(".env")
}

// LoadEnvFile reads a .env file and sets environment variables that are not already set.
func LoadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return // .env is optional
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("warning: failed to close .env file: %v", err)
		}
	}()

	scanner := bufio.NewScanner(f)
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
		// Don't override existing env vars
		if os.Getenv(key) == "" {
			if err := os.Setenv(key, value); err != nil {
				log.Printf("warning: could not set env var %s: %v", key, err)
			}
		}
	}
}

// GetAPIKey returns the TronGrid API key from the TRONGRID_API_KEY environment variable.
func GetAPIKey() string {
	return os.Getenv("TRONGRID_API_KEY")
}
