package main

import (
	"bufio"
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/handlers"
	"mangahub/services/api/internal/protocols"
)

func main() {
	loadDotEnv(".env", filepath.Join("..", "..", ".env"))
	port := getenv("PORT", "8080")
	tcpPort := getenv("TCP_PORT", "9090")
	udpPort := getenv("UDP_PORT", "9091")
	grpcPort := getenv("GRPC_PORT", "9092")
	websocketPort := getenv("WEBSOCKET_PORT", "9093")
	dbPath := getenv("DATABASE_PATH", "mangahub.db")
	databaseURL := getenv("DATABASE_URL", "")
	jwtSecret := getenv("JWT_SECRET", "")
	if jwtSecret == "" {
		if getenv("GIN_MODE", "") == "release" {
			log.Fatal("JWT_SECRET is required in release mode")
		}
		jwtSecret = "dev-secret-change-before-production"
	}

	db, err := database.OpenConfigured(databaseURL, dbPath)
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	jwtManager := auth.NewJWTManager(jwtSecret)
	allowedOrigin := getenv("ALLOWED_ORIGIN", "http://localhost:3000")
	tcpServer := protocols.NewProgressSyncServer(":"+tcpPort, jwtManager)
	udpServer := protocols.NewNotificationServer(":" + udpPort)
	grpcServer := protocols.NewGRPCServer(":"+grpcPort, db, jwtManager, tcpServer)
	chatServer := protocols.NewChatServer(":"+websocketPort, jwtManager, allowedOrigin)

	protocolStatus := newStatusTracker(map[string]string{
		"tcp":       "configured",
		"udp":       "configured",
		"grpc":      "configured",
		"websocket": "configured",
	})
	startProtocol(ctx, "tcp", protocolStatus, tcpServer.Start)
	startProtocol(ctx, "udp", protocolStatus, udpServer.Start)
	startProtocol(ctx, "grpc", protocolStatus, grpcServer.Start)
	startProtocol(ctx, "websocket", protocolStatus, chatServer.Start)

	chapterStorage, err := handlers.NewConfiguredChapterStorage(handlers.StorageConfig{
		Provider:          getenv("STORAGE_PROVIDER", ""),
		LocalDir:          getenv("CHAPTER_UPLOAD_DIR", ""),
		R2Endpoint:        getenv("R2_ENDPOINT", ""),
		R2AccountID:       getenv("R2_ACCOUNT_ID", ""),
		R2Bucket:          getenv("R2_BUCKET", ""),
		R2AccessKeyID:     getenv("R2_ACCESS_KEY_ID", ""),
		R2SecretAccessKey: getenv("R2_SECRET_ACCESS_KEY", ""),
		R2PublicBaseURL:   getenv("R2_PUBLIC_BASE_URL", ""),
		R2Region:          getenv("R2_REGION", ""),
	})
	if err != nil {
		log.Fatalf("configure chapter storage: %v", err)
	}

	server := handlers.NewServerWithConfig(db, jwtManager, handlers.Config{
		AllowedOrigin:           allowedOrigin,
		AdminSyncToken:          getenv("ADMIN_SYNC_TOKEN", ""),
		AdminEmails:             splitCSV(getenv("ADMIN_EMAILS", "")),
		ChapterUploadDir:        getenv("CHAPTER_UPLOAD_DIR", ""),
		ChapterStorage:          chapterStorage,
		ProgressBroadcaster:     tcpServer,
		NotificationBroadcaster: udpServer,
		ServiceStatusFunc:       protocolStatus.snapshot,
	})
	if bootstrapStatuses := splitCSV(getenv("ANILIST_BOOTSTRAP_STATUSES", "")); len(bootstrapStatuses) > 0 {
		bootstrapLimit := clampEnvInt(getenv("ANILIST_BOOTSTRAP_LIMIT", "25"), 1, 50)
		go func() {
			log.Printf("AniList metadata bootstrap starting: statuses=%s limit=%d rights_status=metadata-only", strings.Join(bootstrapStatuses, ","), bootstrapLimit)
			stats, total, err := handlers.SyncAniListCatalog(db, "", bootstrapStatuses, bootstrapLimit)
			if err != nil {
				log.Printf("AniList metadata bootstrap failed: %v", err)
				return
			}
			server.InvalidateCatalogCache()
			log.Printf("AniList metadata bootstrap synced %d manga: %+v", total, stats)
		}()
	}
	if bootstrapStatuses := splitCSV(getenv("MANGADEX_BOOTSTRAP_STATUSES", "")); len(bootstrapStatuses) > 0 {
		bootstrapLimit := clampEnvInt(getenv("MANGADEX_BOOTSTRAP_LIMIT", "25"), 1, 100)
		go func() {
			log.Printf("MangaDex metadata bootstrap starting: statuses=%s limit=%d rights_status=metadata-only", strings.Join(bootstrapStatuses, ","), bootstrapLimit)
			stats, total, err := handlers.SyncMangaDexCatalog(db, "", bootstrapStatuses, bootstrapLimit)
			if err != nil {
				log.Printf("MangaDex metadata bootstrap failed: %v", err)
				return
			}
			server.InvalidateCatalogCache()
			log.Printf("MangaDex metadata bootstrap synced %d manga: %+v", total, stats)
		}()
	}

	httpServer := &http.Server{Addr: ":" + port, Handler: server.Router}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("MangaHub HTTP API listening on http://localhost:%s", port)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("run server: %v", err)
	}
}

type statusTracker struct {
	mu     sync.RWMutex
	values map[string]string
}

func newStatusTracker(initial map[string]string) *statusTracker {
	values := make(map[string]string, len(initial))
	for key, value := range initial {
		values[key] = value
	}
	return &statusTracker{values: values}
}

func (s *statusTracker) set(name, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[name] = status
}

func (s *statusTracker) snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	values := make(map[string]string, len(s.values))
	for key, value := range s.values {
		values[key] = value
	}
	return values
}

func startProtocol(ctx context.Context, name string, status *statusTracker, run func(context.Context) error) {
	go func() {
		status.set(name, "online")
		if err := run(ctx); err != nil {
			status.set(name, "error")
			log.Printf("%s service stopped: %v", name, err)
			return
		}
		status.set(name, "stopped")
	}()
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == ' '
	})
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return values
}

func clampEnvInt(value string, minValue, maxValue int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return minValue
	}
	if parsed < minValue {
		return minValue
	}
	if parsed > maxValue {
		return maxValue
	}
	return parsed
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
