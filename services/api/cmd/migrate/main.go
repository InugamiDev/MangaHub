package main

import (
	"bufio"
	"log"
	"os"
	"path/filepath"
	"strings"

	"mangahub/services/api/internal/database"
)

func main() {
	loadDotEnv(".env", filepath.Join("..", "..", ".env"))

	db, err := database.OpenConfigured(getenv("DATABASE_URL", ""), getenv("DATABASE_PATH", "mangahub.db"))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := database.SeedManga(db, "data/manga.json"); err != nil {
		log.Fatalf("seed manga: %v", err)
	}

	log.Printf("database migration complete: dialect=%s", db.Dialect)
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func loadDotEnv(paths ...string) {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
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
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if key != "" && os.Getenv(key) == "" {
				_ = os.Setenv(key, value)
			}
		}
		_ = file.Close()
	}
}
