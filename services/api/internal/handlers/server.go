package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	Router                  *gin.Engine
	DB                      *database.Store
	JWT                     *auth.JWTManager
	AllowedOrigin           string
	AdminSyncToken          string
	ProgressBroadcaster     ProgressBroadcaster
	NotificationBroadcaster NotificationBroadcaster
	ServiceStatus           map[string]string
	ServiceStatusFunc       func() map[string]string
	ChapterUploadDir        string
	ChapterStorage          ChapterPageStorage
	statusMu                sync.RWMutex
	catalogCacheMu          sync.RWMutex
	catalogCache            map[string]cachedMangaSearch
	adminEmails             map[string]bool
}

type mangaSearchResponse struct {
	Results []models.Manga `json:"results"`
	Count   int            `json:"count"`
}

type cachedMangaSearch struct {
	response  mangaSearchResponse
	expiresAt time.Time
}

type Config struct {
	AllowedOrigin           string
	AdminSyncToken          string
	AdminEmails             []string
	ProgressBroadcaster     ProgressBroadcaster
	NotificationBroadcaster NotificationBroadcaster
	ServiceStatus           map[string]string
	ServiceStatusFunc       func() map[string]string
	ChapterUploadDir        string
	ChapterStorage          ChapterPageStorage
}

type ProgressBroadcaster interface {
	BroadcastProgress(models.ProgressUpdate) int
}

type ProgressQueueReporter interface {
	QueuedProgressCount(userID string) int
}

type NotificationBroadcaster interface {
	BroadcastNotification(models.Notification) int
}

var aniListCache sync.Map

var aniListStatusAliases = map[string]string{
	"":                 "",
	"all":              "",
	"any":              "",
	"completed":        "FINISHED",
	"complete":         "FINISHED",
	"finished":         "FINISHED",
	"fin":              "FINISHED",
	"ongoing":          "RELEASING",
	"releasing":        "RELEASING",
	"current":          "RELEASING",
	"hiatus":           "HIATUS",
	"on-hiatus":        "HIATUS",
	"cancelled":        "CANCELLED",
	"canceled":         "CANCELLED",
	"not-yet-released": "NOT_YET_RELEASED",
	"not_yet_released": "NOT_YET_RELEASED",
}

func NewServer(db *database.Store, jwt *auth.JWTManager) *Server {
	return NewServerWithConfig(db, jwt, Config{AllowedOrigin: "*"})
}

func NewServerWithConfig(db *database.Store, jwt *auth.JWTManager, config Config) *Server {
	if config.AllowedOrigin == "" {
		config.AllowedOrigin = "*"
	}
	if config.ChapterUploadDir == "" {
		config.ChapterUploadDir = "data/uploads/chapters"
	}
	if config.ChapterStorage == nil {
		config.ChapterStorage = LocalChapterStorage{Dir: config.ChapterUploadDir}
	}
	router := gin.Default()
	server := &Server{
		Router:                  router,
		DB:                      db,
		JWT:                     jwt,
		AllowedOrigin:           config.AllowedOrigin,
		AdminSyncToken:          config.AdminSyncToken,
		ProgressBroadcaster:     config.ProgressBroadcaster,
		NotificationBroadcaster: config.NotificationBroadcaster,
		ServiceStatus:           cloneStatusMap(config.ServiceStatus),
		ServiceStatusFunc:       config.ServiceStatusFunc,
		ChapterUploadDir:        config.ChapterUploadDir,
		ChapterStorage:          config.ChapterStorage,
		catalogCache:            map[string]cachedMangaSearch{},
		adminEmails:             normalizeEmailSet(config.AdminEmails),
	}
	router.Use(cors(config.AllowedOrigin))
	if local, ok := config.ChapterStorage.(LocalChapterStorage); ok {
		router.Static("/media/chapters", local.Dir)
	}

	router.GET("/health", server.health)
	router.POST("/auth/register", server.register)
	router.POST("/auth/login", server.login)
	router.POST("/auth/recovery/request", server.requestPasswordRecovery)
	router.POST("/auth/recovery/reset", server.resetPassword)
	router.GET("/manga", server.searchManga)
	router.GET("/manga/:id/reviews", server.listReviews)
	router.GET("/manga/:id", server.getManga)
	router.GET("/manga/:id/chapters", server.listChapters)
	router.GET("/manga/:id/chapters/:chapterId", server.getChapter)
	router.GET("/sources", server.sources)
	router.GET("/sources/anilist/search", server.searchAniList)
	router.GET("/sources/anilist/:id", server.getAniList)
	router.GET("/sources/mangadex/search", server.searchMangaDex)
	router.GET("/sources/mangadex/cover/:mangaID/:fileName", server.proxyMangaDexCover)
	router.GET("/sources/mangadex/:id", server.getMangaDex)

	admin := router.Group("/admin")
	admin.Use(server.adminSyncMiddleware())
	admin.POST("/sync/anilist", server.syncAniList)
	admin.POST("/sync/mangadex", server.syncMangaDex)
	admin.POST("/import/mangadex/:id", server.importMangaDex)
	admin.POST("/notify", server.notifyClients)
	admin.POST("/manga", server.createManga)
	admin.PUT("/manga/:id", server.updateManga)
	admin.DELETE("/manga/:id", server.deleteManga)
	admin.POST("/manga/:id/chapters", server.createChapter)
	admin.PUT("/manga/:id/chapters/:chapterId", server.updateChapter)
	admin.DELETE("/manga/:id/chapters/:chapterId", server.deleteChapter)

	protected := router.Group("/users")
	protected.Use(jwt.Middleware())
	protected.GET("/me", server.getCurrentUser)
	protected.POST("/library", server.addLibrary)
	protected.GET("/library", server.getLibrary)
	protected.DELETE("/library/:mangaID", server.deleteLibrary)
	protected.PUT("/progress", server.updateProgress)
	protected.GET("/stats", server.getUserStats)
	protected.POST("/friends/request", server.requestFriend)
	protected.POST("/friends/respond", server.respondFriend)
	protected.GET("/friends", server.listFriends)
	protected.GET("/activity", server.friendActivity)

	// intent: keep community review writes authenticated while public readers can view summaries
	// status: done
	// next: add moderation roles if reviews need approval workflow
	// blockers: none
	// confidence: high
	mangaProtected := router.Group("/manga")
	mangaProtected.Use(jwt.Middleware())
	mangaProtected.POST("/:id/reviews", server.submitReview)

	return server
}

func cors(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if corsOriginAllowed(allowedOrigin, origin) {
			if strings.TrimSpace(allowedOrigin) == "*" && origin == "" {
				c.Header("Access-Control-Allow-Origin", "*")
			} else if origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Vary", "Origin")
			}
		}
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Admin-Sync-Token")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func corsOriginAllowed(allowedOrigin string, origin string) bool {
	allowedOrigin = strings.TrimSpace(allowedOrigin)
	origin = strings.TrimSpace(origin)
	if origin == "" || allowedOrigin == "" || allowedOrigin == "*" {
		return true
	}
	for _, allowed := range strings.Split(allowedOrigin, ",") {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if allowed == "*" || allowed == origin || localLoopbackOriginMatch(allowed, origin) {
			return true
		}
	}
	return false
}

func localLoopbackOriginMatch(allowed string, origin string) bool {
	allowedURL, allowedErr := url.Parse(allowed)
	originURL, originErr := url.Parse(origin)
	if allowedErr != nil || originErr != nil {
		return false
	}
	if allowedURL.Scheme != originURL.Scheme || allowedURL.Port() != originURL.Port() {
		return false
	}
	return isLoopbackHost(allowedURL.Hostname()) && isLoopbackHost(originURL.Hostname())
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(strings.Trim(host, "[]")) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func (s *Server) health(c *gin.Context) {
	if err := s.DB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "database": "unavailable"})
		return
	}
	services := gin.H{
		"http":      "online",
		"tcp":       "not_configured",
		"udp":       "not_configured",
		"grpc":      "not_configured",
		"websocket": "not_configured",
	}
	for name, status := range s.currentServiceStatus() {
		services[name] = status
	}
	c.JSON(http.StatusOK, gin.H{
		"status":   "healthy",
		"database": gin.H{"status": "active", "dialect": s.DB.Dialect},
		"services": services,
	})
}

func (s *Server) currentServiceStatus() map[string]string {
	if s.ServiceStatusFunc != nil {
		return cloneStatusMap(s.ServiceStatusFunc())
	}
	s.statusMu.RLock()
	defer s.statusMu.RUnlock()
	return cloneStatusMap(s.ServiceStatus)
}

func (s *Server) SetServiceStatus(name, status string) {
	name = strings.TrimSpace(name)
	status = strings.TrimSpace(status)
	if name == "" || status == "" {
		return
	}
	s.statusMu.Lock()
	defer s.statusMu.Unlock()
	if s.ServiceStatus == nil {
		s.ServiceStatus = make(map[string]string)
	}
	s.ServiceStatus[name] = status
}

func cloneStatusMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return nil
	}
	clone := make(map[string]string, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

type registerRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if err := validateRegistration(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	userID := "usr_" + uuid.NewString()
	role := "reader"
	if s.adminEmails[req.Email] {
		role = "admin"
	}
	_, err = s.DB.Exec(`INSERT INTO users (id, username, email, password_hash, role) VALUES (?, ?, ?, ?, ?)`, userID, req.Username, req.Email, hash, role)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username or email already exists"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"user": gin.H{"id": userID, "username": req.Username, "email": req.Email, "role": role},
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	identifier := strings.TrimSpace(req.Username)
	column := "username"
	if identifier == "" {
		identifier = strings.TrimSpace(strings.ToLower(req.Email))
		column = "email"
	}
	if identifier == "" || req.Password == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username/email and password are required"})
		return
	}

	var userID, username, email, passwordHash, role string
	query := `SELECT id, username, email, password_hash, role FROM users WHERE ` + column + ` = ?`
	if err := s.DB.QueryRow(query, identifier).Scan(&userID, &username, &email, &passwordHash, &role); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	if !auth.CheckPassword(passwordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	role = normalizeUserRole(role)
	if s.adminEmails[strings.ToLower(email)] && role != "admin" {
		role = "admin"
		if _, err := s.DB.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user role"})
			return
		}
	}
	token, expiresAt, err := s.JWT.Generate(userID, username, role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt,
		"user":       gin.H{"id": userID, "username": username, "email": email, "role": role},
	})
}

func (s *Server) searchManga(c *gin.Context) {
	searchTerm := strings.ToLower(strings.TrimSpace(c.Query("q")))
	genre := strings.TrimSpace(strings.ToLower(c.Query("genre")))
	status := normalizeCatalogStatus(c.Query("status"))
	limit := clampInt(c.DefaultQuery("limit", "24"), 1, 500)
	offset := clampInt(c.DefaultQuery("offset", "0"), 0, 10000)
	year := clampInt(c.DefaultQuery("year", "0"), 0, 9999)
	minRating := clampFloat(c.DefaultQuery("min_rating", "0"), 0, 5)
	sortMode := normalizeMangaSort(c.Query("sort"))
	cacheKey := strings.Join([]string{searchTerm, genre, status, strconv.Itoa(limit), strconv.Itoa(offset), strconv.Itoa(year), fmt.Sprintf("%.2f", minRating), sortMode}, "\x00")
	if cached, ok := s.getCatalogCache(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	query := "%" + searchTerm + "%"
	orderBy := mangaSearchOrderBy(sortMode)

	rows, err := s.DB.Query(`
SELECT m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url, m.source_provider, m.source_url, m.rights_status, m.publication_year,
       COALESCE(AVG(r.rating), 0), COUNT(r.id)
FROM manga m
LEFT JOIN reviews r ON r.manga_id = m.id
WHERE (lower(m.title) LIKE ? OR lower(m.author) LIKE ? OR lower(m.description) LIKE ?)
AND (? = '' OR lower(m.status) = ?)
AND (? = 0 OR m.publication_year = ?)
GROUP BY m.id, m.title, m.author, m.genres, m.status, m.total_chapters, m.description, m.cover_url, m.source_provider, m.source_url, m.rights_status, m.publication_year
HAVING (? = 0 OR COALESCE(AVG(r.rating), 0) >= ?)
ORDER BY `+orderBy+`
LIMIT ? OFFSET ?`, query, query, query, status, status, year, year, minRating, minRating, limit, offset)
	if err != nil {
		log.Printf("search manga query failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	defer rows.Close()

	results := make([]models.Manga, 0)
	for rows.Next() {
		manga, err := scanMangaWithStats(rows)
		if err != nil {
			log.Printf("search manga scan failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid manga record"})
			return
		}
		if genre == "" || hasGenre(manga.Genres, genre) {
			manga = s.repairCatalogSourceCover(manga)
			results = append(results, manga)
		}
	}
	response := mangaSearchResponse{Results: results, Count: len(results)}
	s.setCatalogCache(cacheKey, response)
	c.JSON(http.StatusOK, response)
}

func (s *Server) getCatalogCache(key string) (mangaSearchResponse, bool) {
	s.catalogCacheMu.RLock()
	entry, ok := s.catalogCache[key]
	s.catalogCacheMu.RUnlock()
	if !ok || time.Now().After(entry.expiresAt) {
		if ok {
			s.catalogCacheMu.Lock()
			delete(s.catalogCache, key)
			s.catalogCacheMu.Unlock()
		}
		return mangaSearchResponse{}, false
	}
	return entry.response, true
}

func (s *Server) setCatalogCache(key string, response mangaSearchResponse) {
	s.catalogCacheMu.Lock()
	s.catalogCache[key] = cachedMangaSearch{
		response:  response,
		expiresAt: time.Now().Add(30 * time.Second),
	}
	s.catalogCacheMu.Unlock()
}

func (s *Server) invalidateCatalogCache() {
	s.catalogCacheMu.Lock()
	s.catalogCache = map[string]cachedMangaSearch{}
	s.catalogCacheMu.Unlock()
}

func (s *Server) InvalidateCatalogCache() {
	s.invalidateCatalogCache()
}

func (s *Server) repairCatalogSourceCover(manga models.Manga) models.Manga {
	manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
	manga.CoverLargeURL = models.SanitizeCoverURL(manga.CoverLargeURL)

	mangaDexID := extractMangaDexMangaID(manga.SourceURL)
	if mangaDexID == "" {
		return manga
	}

	if normalizedCover := normalizeMangaDexCoverURLForID(manga.CoverURL, mangaDexID); normalizedCover != "" {
		if normalizedCover != manga.CoverURL {
			manga.CoverURL = normalizedCover
			s.persistMangaDexCoverRepair(manga)
		}
		return manga
	}

	enriched, err := cachedMangaDexManga(mangaDexID)
	if err != nil {
		return manga
	}
	repaired := mergeMangaDexSource(manga, enriched)
	if !isMangaDexCoverURLForID(repaired.CoverURL, mangaDexID) {
		return manga
	}

	s.persistMangaDexCoverRepair(repaired)
	return repaired
}

func (s *Server) persistMangaDexCoverRepair(manga models.Manga) {
	if _, err := s.DB.Exec(`
UPDATE manga
SET cover_url = ?, source_provider = ?, rights_status = ?
WHERE id = ?`, manga.CoverURL, manga.SourceProvider, manga.RightsStatus, manga.ID); err != nil {
		log.Printf("repair MangaDex cover for %q failed: %v", manga.ID, err)
	}
}

func (s *Server) getManga(c *gin.Context) {
	row := s.DB.QueryRow(`
SELECT id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status, publication_year
FROM manga WHERE id = ?`, c.Param("id"))
	manga, err := scanManga(row)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	if err != nil {
		log.Printf("get manga %q failed: %v", c.Param("id"), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load manga"})
		return
	}
	s.attachReviewSummary(&manga)
	c.JSON(http.StatusOK, enrichMangaFromSource(manga))
}

func (s *Server) sources(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"sources": []gin.H{
			{
				"id":            "internal",
				"name":          "MangaHub catalog",
				"kind":          "curated metadata",
				"rights_status": "metadata-only",
				"endpoint":      "/manga",
			},
			{
				"id":            "anilist",
				"name":          "AniList GraphQL",
				"kind":          "public manga metadata",
				"rights_status": "metadata-only; no chapter page assets are imported",
				"endpoint":      "/sources/anilist/:id",
				"search":        "/sources/anilist/search",
				"statuses":      []string{"completed", "ongoing", "hiatus", "cancelled", "not-yet-released"},
			},
			{
				"id":            "mangadex",
				"name":          "MangaDex API",
				"kind":          "public manga metadata and title provenance",
				"rights_status": "metadata-only; covers are referenced by URL and chapter pages are not mirrored into R2",
				"endpoint":      "/sources/mangadex/:id",
				"search":        "/sources/mangadex/search",
				"statuses":      []string{"completed", "ongoing", "hiatus", "cancelled"},
			},
			{
				"id":            "manga-plus",
				"name":          "MANGA Plus by SHUEISHA",
				"kind":          "official reader links",
				"rights_status": "link-out only",
				"endpoint":      "https://mangaplus.shueisha.co.jp/updates",
			},
		},
	})
}

func (s *Server) searchMangaDex(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	status, ok := normalizeMangaDexRequestStatus(c.Query("status"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be completed, ongoing, hiatus, cancelled, or a valid MangaDex status"})
		return
	}
	limit := clampInt(c.DefaultQuery("limit", "12"), 1, 100)
	results, err := fetchMangaDexMangaWithStatus(query, status, limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source lookup failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
		"source": gin.H{
			"provider":      "MangaDex API",
			"rights_status": "metadata-only",
		},
	})
}

func (s *Server) getMangaDex(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if !validMangaDexID(id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid MangaDex manga id is required"})
		return
	}
	result, err := cachedMangaDexManga(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source lookup failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": result,
		"source": gin.H{
			"provider":      "MangaDex API",
			"rights_status": "metadata-only",
		},
	})
}

func (s *Server) searchAniList(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	status, ok := normalizeAniListRequestStatus(c.Query("status"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be completed, ongoing, hiatus, cancelled, or a valid AniList media status"})
		return
	}
	limit := clampInt(c.DefaultQuery("limit", "12"), 1, 50)
	results, err := fetchAniListMangaWithStatus(query, status, limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source lookup failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"count":   len(results),
		"source": gin.H{
			"provider":      "AniList GraphQL",
			"rights_status": "metadata-only",
		},
	})
}

func (s *Server) getAniList(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid AniList manga id is required"})
		return
	}
	result, err := cachedAniListManga(id)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source lookup failed"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"result": result,
		"source": gin.H{
			"provider":      "AniList GraphQL",
			"rights_status": "metadata-only",
		},
	})
}

type syncAniListRequest struct {
	Query    string   `json:"query"`
	Status   string   `json:"status"`
	Statuses []string `json:"statuses"`
	Limit    int      `json:"limit"`
}

type syncMangaDexRequest struct {
	Query    string   `json:"query"`
	Status   string   `json:"status"`
	Statuses []string `json:"statuses"`
	Limit    int      `json:"limit"`
}

func (s *Server) syncAniList(c *gin.Context) {
	var req syncAniListRequest
	_ = c.ShouldBindJSON(&req)
	limit := req.Limit
	if limit == 0 {
		limit = 35
	}
	statuses := append([]string{}, req.Statuses...)
	if strings.TrimSpace(req.Status) != "" {
		statuses = append(statuses, req.Status)
	}
	if len(statuses) == 0 {
		statuses = append(statuses, "")
	}
	stats, total, err := syncAniListCatalog(s.DB, strings.TrimSpace(req.Query), statuses, clampInt(strconv.Itoa(limit), 1, 50))
	if err != nil {
		if strings.Contains(err.Error(), "invalid AniList status") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source sync failed"})
		return
	}
	if total == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist manga sync"})
		return
	}
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, gin.H{
		"status":     "synced",
		"count":      total,
		"per_status": stats,
		"source": gin.H{
			"provider":      "AniList GraphQL",
			"rights_status": "metadata-only",
		},
	})
}

func (s *Server) syncMangaDex(c *gin.Context) {
	var req syncMangaDexRequest
	_ = c.ShouldBindJSON(&req)
	limit := req.Limit
	if limit == 0 {
		limit = 35
	}
	statuses := append([]string{}, req.Statuses...)
	if strings.TrimSpace(req.Status) != "" {
		statuses = append(statuses, req.Status)
	}
	if len(statuses) == 0 {
		statuses = append(statuses, "")
	}
	stats, total, err := syncMangaDexCatalog(s.DB, strings.TrimSpace(req.Query), statuses, clampInt(strconv.Itoa(limit), 1, 100))
	if err != nil {
		if strings.Contains(err.Error(), "invalid MangaDex status") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": "legal source sync failed"})
		return
	}
	if total == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist manga sync"})
		return
	}
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, gin.H{
		"status":     "synced",
		"count":      total,
		"per_status": stats,
		"source": gin.H{
			"provider":      "MangaDex API",
			"rights_status": "metadata-only",
		},
	})
}

type notifyRequest struct {
	MangaID string `json:"manga_id"`
	Message string `json:"message"`
	Type    string `json:"type"`
}

func (s *Server) notifyClients(c *gin.Context) {
	if s.NotificationBroadcaster == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "UDP notification service is unavailable"})
		return
	}
	var req notifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.MangaID = strings.TrimSpace(req.MangaID)
	req.Message = strings.TrimSpace(req.Message)
	notificationType := strings.TrimSpace(req.Type)
	if notificationType == "" {
		notificationType = "chapter_release"
	}
	if req.MangaID == "" || req.Message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "manga_id and message are required"})
		return
	}
	notification := models.Notification{
		Type:      notificationType,
		MangaID:   req.MangaID,
		Message:   req.Message,
		Timestamp: time.Now().Unix(),
	}
	count := s.NotificationBroadcaster.BroadcastNotification(notification)
	c.JSON(http.StatusOK, gin.H{
		"status":        "sent",
		"delivered":     count,
		"notification":  notification,
		"transport":     "udp",
		"rights_status": "metadata-only",
	})
}

func (s *Server) adminSyncMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.authorizeAdminBearer(c) {
			c.Next()
			return
		}
		token := strings.TrimSpace(c.GetHeader("X-Admin-Sync-Token"))
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin role or ADMIN_SYNC_TOKEN is required"})
			return
		}
		if s.AdminSyncToken == "" || token != s.AdminSyncToken {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin role is required"})
			return
		}
		c.Set("role", "admin")
		c.Next()
	}
}

func (s *Server) authorizeAdminBearer(c *gin.Context) bool {
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		return false
	}
	claims, err := s.JWT.Parse(strings.TrimSpace(strings.TrimPrefix(header, "Bearer ")))
	if err != nil {
		return false
	}
	role, err := s.userRole(claims.UserID)
	if err != nil || role != "admin" {
		return false
	}
	c.Set("user_id", claims.UserID)
	c.Set("username", claims.Username)
	c.Set("role", role)
	return true
}

func (s *Server) userRole(userID string) (string, error) {
	var role string
	if err := s.DB.QueryRow(`SELECT role FROM users WHERE id = ?`, userID).Scan(&role); err != nil {
		return "", err
	}
	return normalizeUserRole(role), nil
}

func (s *Server) getCurrentUser(c *gin.Context) {
	userID := c.GetString("user_id")
	var user models.User
	if err := s.DB.QueryRow(`SELECT id, username, email, role, created_at FROM users WHERE id = ?`, userID).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.CreatedAt); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	user.Role = normalizeUserRole(user.Role)
	c.JSON(http.StatusOK, gin.H{"user": user})
}

type libraryRequest struct {
	MangaID        string `json:"manga_id"`
	Status         string `json:"status"`
	CurrentChapter int    `json:"current_chapter"`
}

func (s *Server) addLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	var req libraryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	if err := validateLibraryRequest(s.DB, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	_, err := s.DB.Exec(`
INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at)
VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(user_id, manga_id) DO UPDATE SET
	current_chapter = excluded.current_chapter,
	status = excluded.status,
	updated_at = CURRENT_TIMESTAMP`, userID, req.MangaID, req.CurrentChapter, req.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to add manga to library"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved"})
}

func (s *Server) getLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := s.DB.Query(`
SELECT m.id, m.title, m.author, m.genres, p.status, p.current_chapter, m.total_chapters,
       m.cover_url, m.source_provider, m.source_url, m.rights_status, p.updated_at
FROM user_progress p
JOIN manga m ON m.id = p.manga_id
WHERE p.user_id = ?
ORDER BY p.updated_at DESC`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load library"})
		return
	}
	defer rows.Close()

	entries := make([]models.LibraryEntry, 0)
	for rows.Next() {
		var entry models.LibraryEntry
		var genresJSON string
		var updated any
		if err := rows.Scan(
			&entry.MangaID,
			&entry.Title,
			&entry.Author,
			&genresJSON,
			&entry.ReadingStatus,
			&entry.CurrentChapter,
			&entry.TotalChapters,
			&entry.CoverURL,
			&entry.SourceProvider,
			&entry.SourceURL,
			&entry.RightsStatus,
			&updated,
		); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid library record"})
			return
		}
		_ = json.Unmarshal([]byte(genresJSON), &entry.Genres)
		enriched := enrichMangaFromSource(models.Manga{
			ID:             entry.MangaID,
			Title:          entry.Title,
			Author:         entry.Author,
			Genres:         entry.Genres,
			Status:         entry.ReadingStatus,
			TotalChapters:  entry.TotalChapters,
			CoverURL:       entry.CoverURL,
			SourceProvider: entry.SourceProvider,
			SourceURL:      entry.SourceURL,
			RightsStatus:   entry.RightsStatus,
		})
		entry.CoverURL = models.SanitizeCoverURL(enriched.CoverURL)
		entry.SourceProvider = enriched.SourceProvider
		entry.SourceURL = enriched.SourceURL
		entry.RightsStatus = enriched.RightsStatus
		entry.UpdatedAt = parseSQLTime(updated)
		entries = append(entries, entry)
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries, "count": len(entries)})
}

type progressRequest struct {
	MangaID string `json:"manga_id"`
	Chapter int    `json:"chapter"`
	Status  string `json:"status"`
}

func (s *Server) updateProgress(c *gin.Context) {
	userID := c.GetString("user_id")
	var req progressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	status := req.Status
	if status == "" {
		status = "reading"
	}
	libraryReq := libraryRequest{MangaID: req.MangaID, Status: status, CurrentChapter: req.Chapter}
	if err := validateLibraryRequest(s.DB, libraryReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := s.DB.Exec(`
UPDATE user_progress
SET current_chapter = ?, status = ?, updated_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND manga_id = ?`, req.Chapter, status, userID, req.MangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update progress"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga is not in your library"})
		return
	}
	progress := models.ProgressUpdate{
		UserID:    userID,
		MangaID:   req.MangaID,
		Chapter:   req.Chapter,
		Timestamp: time.Now().Unix(),
	}
	// intent: surface TCP downtime behavior without coupling HTTP handlers to the TCP server
	// status: done
	// next: extend protobuf response fields if queued status must be exposed over gRPC too
	// blockers: none
	// confidence: high
	broadcastStatus := "not_queued_no_broadcaster"
	broadcastCount := 0
	broadcastQueueCount := 0
	if s.ProgressBroadcaster != nil {
		broadcastCount = s.ProgressBroadcaster.BroadcastProgress(progress)
		broadcastStatus = "sent"
		if broadcastCount == 0 {
			broadcastStatus = "no_tcp_clients"
			if reporter, ok := s.ProgressBroadcaster.(ProgressQueueReporter); ok {
				broadcastQueueCount = reporter.QueuedProgressCount(userID)
			}
			if broadcastQueueCount > 0 {
				broadcastStatus = "queued"
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "updated",
		"sync": gin.H{
			"local_database":      "updated",
			"tcp_broadcast":       broadcastStatus,
			"tcp_broadcast_count": broadcastCount,
			"tcp_queue_count":     broadcastQueueCount,
		},
		"progress": progress,
	})
}

type aniListResponse struct {
	Data struct {
		Page struct {
			Media []aniListMedia `json:"media"`
		} `json:"Page"`
	} `json:"data"`
}

type aniListMediaResponse struct {
	Data struct {
		Media aniListMedia `json:"Media"`
	} `json:"data"`
}

type aniListMedia struct {
	ID    int `json:"id"`
	Title struct {
		UserPreferred string `json:"userPreferred"`
		Romaji        string `json:"romaji"`
		English       string `json:"english"`
		Native        string `json:"native"`
	} `json:"title"`
	Chapters        *int     `json:"chapters"`
	Volumes         *int     `json:"volumes"`
	Status          string   `json:"status"`
	Format          string   `json:"format"`
	Description     string   `json:"description"`
	Genres          []string `json:"genres"`
	Synonyms        []string `json:"synonyms"`
	Source          string   `json:"source"`
	BannerImage     string   `json:"bannerImage"`
	MeanScore       int      `json:"meanScore"`
	AverageScore    int      `json:"averageScore"`
	Popularity      int      `json:"popularity"`
	Favourites      int      `json:"favourites"`
	Hashtag         string   `json:"hashtag"`
	CountryOfOrigin string   `json:"countryOfOrigin"`
	IsLicensed      bool     `json:"isLicensed"`
	CoverImage      struct {
		ExtraLarge string `json:"extraLarge"`
		Large      string `json:"large"`
	} `json:"coverImage"`
	StartDate aniListDate `json:"startDate"`
	EndDate   aniListDate `json:"endDate"`
	SiteURL   string      `json:"siteUrl"`
	Relations struct {
		Edges []struct {
			ID           int    `json:"id"`
			RelationType string `json:"relationType"`
			Node         struct {
				ID    int `json:"id"`
				Title struct {
					UserPreferred string `json:"userPreferred"`
				} `json:"title"`
				Format      string `json:"format"`
				Type        string `json:"type"`
				Status      string `json:"status"`
				BannerImage string `json:"bannerImage"`
				CoverImage  struct {
					Large string `json:"large"`
				} `json:"coverImage"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"relations"`
	CharacterPreview struct {
		Edges []struct {
			ID   int    `json:"id"`
			Role string `json:"role"`
			Node struct {
				ID   int `json:"id"`
				Name struct {
					UserPreferred string `json:"userPreferred"`
				} `json:"name"`
				Image struct {
					Large string `json:"large"`
				} `json:"image"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"characterPreview"`
	StaffPreview struct {
		Edges []struct {
			ID   int    `json:"id"`
			Role string `json:"role"`
			Node struct {
				ID   int `json:"id"`
				Name struct {
					UserPreferred string `json:"userPreferred"`
					Full          string `json:"full"`
				} `json:"name"`
				Language string `json:"language"`
				Image    struct {
					Large string `json:"large"`
				} `json:"image"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"staffPreview"`
	Staff struct {
		Nodes []struct {
			Name struct {
				Full string `json:"full"`
			} `json:"name"`
		} `json:"nodes"`
	} `json:"staff"`
	Recommendations struct {
		Nodes []struct {
			ID                  int    `json:"id"`
			Rating              int    `json:"rating"`
			UserRating          string `json:"userRating"`
			MediaRecommendation struct {
				ID    int `json:"id"`
				Title struct {
					UserPreferred string `json:"userPreferred"`
				} `json:"title"`
				Format      string `json:"format"`
				Type        string `json:"type"`
				Status      string `json:"status"`
				BannerImage string `json:"bannerImage"`
				CoverImage  struct {
					Large string `json:"large"`
				} `json:"coverImage"`
			} `json:"mediaRecommendation"`
		} `json:"nodes"`
	} `json:"recommendations"`
	ExternalLinks []struct {
		ID         int    `json:"id"`
		Site       string `json:"site"`
		URL        string `json:"url"`
		Type       string `json:"type"`
		Language   string `json:"language"`
		Color      string `json:"color"`
		Icon       string `json:"icon"`
		Notes      string `json:"notes"`
		IsDisabled bool   `json:"isDisabled"`
	} `json:"externalLinks"`
	Rankings []struct {
		ID      int    `json:"id"`
		Rank    int    `json:"rank"`
		Type    string `json:"type"`
		Format  string `json:"format"`
		Year    int    `json:"year"`
		Season  string `json:"season"`
		AllTime bool   `json:"allTime"`
		Context string `json:"context"`
	} `json:"rankings"`
	Tags []struct {
		ID               int    `json:"id"`
		Name             string `json:"name"`
		Description      string `json:"description"`
		Rank             int    `json:"rank"`
		IsMediaSpoiler   bool   `json:"isMediaSpoiler"`
		IsGeneralSpoiler bool   `json:"isGeneralSpoiler"`
	} `json:"tags"`
}

type aniListDate struct {
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

func fetchAniListManga(search string, limit int) ([]models.Manga, error) {
	return fetchAniListMangaWithStatus(search, "", limit)
}

func fetchAniListMangaWithStatus(search, status string, limit int) ([]models.Manga, error) {
	const endpoint = "https://graphql.anilist.co"
	query := `
query ($search: String, $perPage: Int, $status: MediaStatus, $isAdult: Boolean) {
	Page(page: 1, perPage: $perPage) {
		media(type: MANGA, search: $search, status: $status, isAdult: $isAdult, sort: POPULARITY_DESC) {
			id
			title { userPreferred romaji english native }
			chapters
			volumes
			status(version: 2)
			format
			description(asHtml: false)
			genres
			synonyms
			meanScore
			averageScore
			popularity
			favourites
			countryOfOrigin
			isLicensed
			coverImage { extraLarge large }
			bannerImage
			siteUrl
			staff(perPage: 1, sort: RELEVANCE) { nodes { name { full } } }
		}
	}
}`
	searchValue := any(nil)
	if strings.TrimSpace(search) != "" {
		searchValue = strings.TrimSpace(search)
	}
	statusValue := any(nil)
	if strings.TrimSpace(status) != "" {
		statusValue = strings.TrimSpace(status)
	}
	variables := gin.H{"perPage": limit, "search": searchValue, "status": statusValue, "isAdult": false}
	body, err := json.Marshal(gin.H{
		"query":     query,
		"variables": variables,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.New("anilist returned non-2xx status")
	}

	var payload aniListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	results := make([]models.Manga, 0, len(payload.Data.Page.Media))
	for _, media := range payload.Data.Page.Media {
		results = append(results, mapAniListMedia(media))
	}
	return results, nil
}

func SyncAniListCatalog(db *database.Store, search string, statuses []string, limit int) (map[string]int, int, error) {
	return syncAniListCatalog(db, search, statuses, limit)
}

func syncAniListCatalog(db *database.Store, search string, statuses []string, limit int) (map[string]int, int, error) {
	if db == nil {
		return nil, 0, errors.New("database is required")
	}
	normalizedStatuses, err := normalizeAniListStatusBuckets(statuses)
	if err != nil {
		return nil, 0, err
	}
	stats := make(map[string]int, len(normalizedStatuses))
	entries := make([]models.Manga, 0, len(normalizedStatuses)*limit)
	seen := make(map[string]bool)
	for _, status := range normalizedStatuses {
		results, err := fetchAniListMangaWithStatus(search, status, limit)
		if err != nil {
			return stats, len(entries), err
		}
		key := aniListStatusResponseKey(status)
		for _, manga := range results {
			if manga.ID == "" || seen[manga.ID] {
				continue
			}
			seen[manga.ID] = true
			manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
			manga.CoverLargeURL = models.SanitizeCoverURL(firstNonEmpty(manga.CoverLargeURL, manga.CoverURL))
			manga.SourceProvider = "AniList GraphQL"
			manga.RightsStatus = "metadata-only"
			entries = append(entries, manga)
			stats[key]++
		}
		if _, ok := stats[key]; !ok {
			stats[key] = 0
		}
	}
	if len(entries) == 0 {
		return stats, 0, nil
	}
	if err := database.UpsertManga(db, entries); err != nil {
		return stats, 0, err
	}
	return stats, len(entries), nil
}

func fetchAniListMangaByID(id int) (models.Manga, error) {
	const endpoint = "https://graphql.anilist.co"
	query := `
query ($id: Int!, $isAdult: Boolean) {
	Media(id: $id, type: MANGA, isAdult: $isAdult) {
		id
		title { userPreferred romaji english native }
		coverImage { extraLarge large }
		bannerImage
		startDate { year month day }
		endDate { year month day }
		description(asHtml: false)
		type
		format
		status(version: 2)
		chapters
		volumes
		genres
		synonyms
		source(version: 3)
		meanScore
		averageScore
		popularity
		favourites
		hashtag
		countryOfOrigin
		isLicensed
		siteUrl
		relations {
			edges {
				id
				relationType(version: 2)
				node {
					id
					title { userPreferred }
					format
					type
					status(version: 2)
					bannerImage
					coverImage { large }
				}
			}
		}
		characterPreview: characters(perPage: 6, sort: [ROLE, RELEVANCE, ID]) {
			edges {
				id
				role
				node {
					id
					name { userPreferred }
					image { large }
				}
			}
		}
		staffPreview: staff(perPage: 8, sort: [RELEVANCE, ID]) {
			edges {
				id
				role
				node {
					id
					name { userPreferred full }
					language: languageV2
					image { large }
				}
			}
		}
		staff(perPage: 1, sort: RELEVANCE) { nodes { name { full } } }
		recommendations(perPage: 7, sort: [RATING_DESC, ID]) {
			nodes {
				id
				rating
				userRating
				mediaRecommendation {
					id
					title { userPreferred }
					format
					type
					status(version: 2)
					bannerImage
					coverImage { large }
				}
			}
		}
		externalLinks {
			id
			site
			url
			type
			language
			color
			icon
			notes
			isDisabled
		}
		rankings {
			id
			rank
			type
			format
			year
			season
			allTime
			context
		}
		tags {
			id
			name
			description
			rank
			isMediaSpoiler
			isGeneralSpoiler
		}
	}
}`
	body, err := json.Marshal(gin.H{
		"query":     query,
		"variables": gin.H{"id": id, "isAdult": false},
	})
	if err != nil {
		return models.Manga{}, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return models.Manga{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return models.Manga{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return models.Manga{}, errors.New("anilist returned non-2xx status")
	}

	var payload aniListMediaResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return models.Manga{}, err
	}
	if payload.Data.Media.ID == 0 {
		return models.Manga{}, errors.New("anilist manga not found")
	}
	return mapAniListMedia(payload.Data.Media), nil
}

func mapAniListMedia(media aniListMedia) models.Manga {
	title := firstNonEmpty(media.Title.English, media.Title.UserPreferred, media.Title.Romaji, media.Title.Native, "Untitled manga")
	author := "Unknown"
	if len(media.Staff.Nodes) > 0 && media.Staff.Nodes[0].Name.Full != "" {
		author = media.Staff.Nodes[0].Name.Full
	}
	chapters := 0
	if media.Chapters != nil {
		chapters = *media.Chapters
	}
	volumes := 0
	if media.Volumes != nil {
		volumes = *media.Volumes
	}
	coverURL := models.SanitizeCoverURL(firstNonEmpty(media.CoverImage.ExtraLarge, media.CoverImage.Large))
	return models.Manga{
		ID:              "anilist-" + strconv.Itoa(media.ID),
		Title:           title,
		Author:          author,
		Genres:          media.Genres,
		Status:          normalizeAniListStatus(media.Status),
		TotalChapters:   chapters,
		Description:     cleanSourceDescription(media.Description),
		CoverURL:        coverURL,
		SourceProvider:  "AniList GraphQL",
		SourceURL:       media.SiteURL,
		RightsStatus:    "metadata-only",
		PublicationYear: media.StartDate.Year,
		CoverLargeURL:   coverURL,
		BannerURL:       models.SanitizeCoverURL(media.BannerImage),
		StartDate:       formatAniListDate(media.StartDate),
		EndDate:         formatAniListDate(media.EndDate),
		Format:          strings.ToLower(media.Format),
		Volumes:         volumes,
		MeanScore:       media.MeanScore,
		AverageScore:    media.AverageScore,
		Popularity:      media.Popularity,
		Favourites:      media.Favourites,
		CountryOfOrigin: media.CountryOfOrigin,
		IsLicensed:      media.IsLicensed,
		Synonyms:        cleanStringList(media.Synonyms, 8),
		Tags:            mapAniListTags(media),
		Relations:       mapAniListRelations(media),
		Recommendations: mapAniListRecommendations(media),
		Characters:      mapAniListCharacters(media),
		Staff:           mapAniListStaff(media),
		ExternalLinks:   mapAniListExternalLinks(media),
		Rankings:        mapAniListRankings(media),
	}
}

func formatAniListDate(date aniListDate) string {
	if date.Year == 0 {
		return ""
	}
	if date.Month == 0 {
		return strconv.Itoa(date.Year)
	}
	if date.Day == 0 {
		return strconv.Itoa(date.Year) + "-" + twoDigit(date.Month)
	}
	return strconv.Itoa(date.Year) + "-" + twoDigit(date.Month) + "-" + twoDigit(date.Day)
}

func twoDigit(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

func cleanStringList(values []string, limit int) []string {
	cleaned := make([]string, 0, len(values))
	seen := make(map[string]bool)
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		key := strings.ToLower(trimmed)
		if trimmed == "" || seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, trimmed)
		if limit > 0 && len(cleaned) >= limit {
			break
		}
	}
	return cleaned
}

func mapAniListTags(media aniListMedia) []models.MangaTag {
	tags := make([]models.MangaTag, 0, len(media.Tags))
	for _, tag := range media.Tags {
		if tag.Name == "" {
			continue
		}
		tags = append(tags, models.MangaTag{
			ID:               tag.ID,
			Name:             tag.Name,
			Description:      cleanSourceDescription(tag.Description),
			Rank:             tag.Rank,
			IsMediaSpoiler:   tag.IsMediaSpoiler,
			IsGeneralSpoiler: tag.IsGeneralSpoiler,
		})
		if len(tags) >= 12 {
			break
		}
	}
	return tags
}

func mapAniListRelations(media aniListMedia) []models.MangaRelation {
	relations := make([]models.MangaRelation, 0, len(media.Relations.Edges))
	for _, edge := range media.Relations.Edges {
		if edge.Node.ID == 0 || edge.Node.Title.UserPreferred == "" {
			continue
		}
		relations = append(relations, models.MangaRelation{
			ID:           edge.Node.ID,
			RelationType: strings.ToLower(edge.RelationType),
			Title:        edge.Node.Title.UserPreferred,
			Format:       strings.ToLower(edge.Node.Format),
			Type:         strings.ToLower(edge.Node.Type),
			Status:       normalizeAniListStatus(edge.Node.Status),
			CoverURL:     models.SanitizeCoverURL(edge.Node.CoverImage.Large),
			BannerURL:    models.SanitizeCoverURL(edge.Node.BannerImage),
		})
		if len(relations) >= 8 {
			break
		}
	}
	return relations
}

func mapAniListRecommendations(media aniListMedia) []models.MangaRecommendation {
	recommendations := make([]models.MangaRecommendation, 0, len(media.Recommendations.Nodes))
	for _, node := range media.Recommendations.Nodes {
		item := node.MediaRecommendation
		if item.ID == 0 || item.Title.UserPreferred == "" {
			continue
		}
		recommendations = append(recommendations, models.MangaRecommendation{
			ID:         item.ID,
			Rating:     node.Rating,
			UserRating: strings.ToLower(node.UserRating),
			Title:      item.Title.UserPreferred,
			Format:     strings.ToLower(item.Format),
			Type:       strings.ToLower(item.Type),
			Status:     normalizeAniListStatus(item.Status),
			CoverURL:   models.SanitizeCoverURL(item.CoverImage.Large),
			BannerURL:  models.SanitizeCoverURL(item.BannerImage),
		})
	}
	return recommendations
}

func mapAniListCharacters(media aniListMedia) []models.MangaCredit {
	characters := make([]models.MangaCredit, 0, len(media.CharacterPreview.Edges))
	for _, edge := range media.CharacterPreview.Edges {
		name := strings.TrimSpace(edge.Node.Name.UserPreferred)
		if edge.Node.ID == 0 || name == "" {
			continue
		}
		characters = append(characters, models.MangaCredit{
			ID:       edge.Node.ID,
			Name:     name,
			Role:     strings.ToLower(edge.Role),
			ImageURL: models.SanitizeCoverURL(edge.Node.Image.Large),
		})
	}
	return characters
}

func mapAniListStaff(media aniListMedia) []models.MangaCredit {
	staff := make([]models.MangaCredit, 0, len(media.StaffPreview.Edges))
	for _, edge := range media.StaffPreview.Edges {
		name := firstNonEmpty(edge.Node.Name.UserPreferred, edge.Node.Name.Full)
		if edge.Node.ID == 0 || name == "" {
			continue
		}
		staff = append(staff, models.MangaCredit{
			ID:       edge.Node.ID,
			Name:     name,
			Role:     edge.Role,
			ImageURL: models.SanitizeCoverURL(edge.Node.Image.Large),
			Language: edge.Node.Language,
		})
	}
	return staff
}

func mapAniListExternalLinks(media aniListMedia) []models.MangaExternalLink {
	links := make([]models.MangaExternalLink, 0, len(media.ExternalLinks))
	for _, link := range media.ExternalLinks {
		if link.IsDisabled || strings.TrimSpace(link.Site) == "" {
			continue
		}
		url := models.SanitizeCoverURL(link.URL)
		if url == "" {
			continue
		}
		links = append(links, models.MangaExternalLink{
			ID:       link.ID,
			Site:     link.Site,
			URL:      url,
			Type:     strings.ToLower(link.Type),
			Language: link.Language,
			Color:    link.Color,
			Icon:     models.SanitizeCoverURL(link.Icon),
			Notes:    link.Notes,
		})
		if len(links) >= 8 {
			break
		}
	}
	return links
}

func mapAniListRankings(media aniListMedia) []models.MangaRanking {
	rankings := make([]models.MangaRanking, 0, len(media.Rankings))
	for _, rank := range media.Rankings {
		if rank.Rank <= 0 || strings.TrimSpace(rank.Context) == "" {
			continue
		}
		rankings = append(rankings, models.MangaRanking{
			ID:      rank.ID,
			Rank:    rank.Rank,
			Type:    strings.ToLower(rank.Type),
			Format:  strings.ToLower(rank.Format),
			Year:    rank.Year,
			Season:  strings.ToLower(rank.Season),
			AllTime: rank.AllTime,
			Context: rank.Context,
		})
		if len(rankings) >= 6 {
			break
		}
	}
	return rankings
}

func enrichMangaFromSource(manga models.Manga) models.Manga {
	if id := extractAniListMangaID(manga.SourceURL); id > 0 {
		enriched, err := cachedAniListManga(id)
		if err == nil {
			return mergeSourceManga(manga, enriched)
		}
	}
	if id := extractMangaDexMangaID(manga.SourceURL); id != "" {
		enriched, err := cachedMangaDexManga(id)
		if err == nil {
			return mergeMangaDexSource(manga, enriched)
		}
	}
	manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
	return manga
}

func cachedAniListManga(id int) (models.Manga, error) {
	if cached, ok := aniListCache.Load(id); ok {
		if manga, ok := cached.(models.Manga); ok {
			return manga, nil
		}
	}
	manga, err := fetchAniListMangaByID(id)
	if err != nil {
		return models.Manga{}, err
	}
	aniListCache.Store(id, manga)
	return manga, nil
}

func mergeSourceManga(base, source models.Manga) models.Manga {
	source.ID = base.ID
	source.Title = firstNonEmpty(base.Title, source.Title)
	source.Author = firstNonEmpty(base.Author, source.Author)
	if len(base.Genres) > 0 {
		source.Genres = base.Genres
	}
	source.Status = firstNonEmpty(base.Status, source.Status)
	if base.TotalChapters > 0 {
		source.TotalChapters = base.TotalChapters
	}
	source.SourceURL = firstNonEmpty(base.SourceURL, source.SourceURL)
	source.SourceProvider = firstNonEmpty(source.SourceProvider, base.SourceProvider)
	source.RightsStatus = firstNonEmpty(source.RightsStatus, base.RightsStatus, "metadata-only")
	source.CoverURL = firstNonEmpty(source.CoverURL, base.CoverURL)
	source.CoverLargeURL = firstNonEmpty(source.CoverLargeURL, source.CoverURL)
	if strings.TrimSpace(source.Description) == "" {
		source.Description = base.Description
	}
	return source
}

func extractAniListMangaID(value string) int {
	matches := regexp.MustCompile(`(?i)anilist\.co/manga/(\d+)`).FindStringSubmatch(value)
	if len(matches) < 2 {
		return 0
	}
	id, _ := strconv.Atoi(matches[1])
	return id
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func normalizeAniListStatus(status string) string {
	switch strings.ToUpper(status) {
	case "FINISHED":
		return "completed"
	case "RELEASING":
		return "ongoing"
	case "HIATUS":
		return "hiatus"
	case "CANCELLED":
		return "cancelled"
	case "NOT_YET_RELEASED":
		return "not-yet-released"
	default:
		return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status), "_", "-"))
	}
}

func normalizeCatalogStatus(status string) string {
	aniListStatus, ok := normalizeAniListRequestStatus(status)
	if !ok {
		return strings.ToLower(strings.TrimSpace(status))
	}
	return normalizeAniListStatus(aniListStatus)
}

func normalizeAniListRequestStatus(status string) (string, bool) {
	trimmed := strings.TrimSpace(status)
	key := strings.ToLower(strings.ReplaceAll(trimmed, "_", "-"))
	if value, ok := aniListStatusAliases[key]; ok {
		return value, true
	}
	upper := strings.ToUpper(strings.ReplaceAll(trimmed, "-", "_"))
	switch upper {
	case "FINISHED", "RELEASING", "HIATUS", "CANCELLED", "NOT_YET_RELEASED":
		return upper, true
	default:
		return "", false
	}
}

func normalizeAniListStatusBuckets(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		statuses = []string{"completed", "ongoing", "hiatus", "cancelled"}
	}
	normalized := make([]string, 0, len(statuses))
	seen := make(map[string]bool)
	for _, status := range statuses {
		value, ok := normalizeAniListRequestStatus(status)
		if !ok {
			return nil, fmt.Errorf("invalid AniList status %q", status)
		}
		if value == "" {
			continue
		}
		if !seen[value] {
			seen[value] = true
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return []string{"FINISHED", "RELEASING", "HIATUS", "CANCELLED"}, nil
	}
	return normalized, nil
}

func aniListStatusResponseKey(status string) string {
	if strings.TrimSpace(status) == "" {
		return "all"
	}
	return normalizeAniListStatus(status)
}

func normalizeEmailSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]bool, len(values))
	for _, value := range values {
		email := strings.ToLower(strings.TrimSpace(value))
		if email != "" {
			normalized[email] = true
		}
	}
	return normalized
}

func normalizeUserRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin":
		return "admin"
	case "moderator":
		return "moderator"
	default:
		return "reader"
	}
}

func cleanSourceDescription(description string) string {
	cleaned := regexp.MustCompile(`<[^>]*>`).ReplaceAllString(description, "")
	cleaned = strings.ReplaceAll(cleaned, "\n", " ")
	cleaned = strings.Join(strings.Fields(cleaned), " ")
	if len(cleaned) > 900 {
		return cleaned[:897] + "..."
	}
	return cleaned
}

func parseSQLTime(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		return parseTimeString(typed)
	case []byte:
		return parseTimeString(string(typed))
	default:
		return time.Time{}
	}
}

func parseTimeString(value string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04:05Z07:00"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanManga(row rowScanner) (models.Manga, error) {
	var manga models.Manga
	var genresJSON string
	err := row.Scan(&manga.ID, &manga.Title, &manga.Author, &genresJSON, &manga.Status, &manga.TotalChapters, &manga.Description, &manga.CoverURL, &manga.SourceProvider, &manga.SourceURL, &manga.RightsStatus, &manga.PublicationYear)
	if err != nil {
		return manga, err
	}
	if err := json.Unmarshal([]byte(genresJSON), &manga.Genres); err != nil {
		return manga, err
	}
	manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
	return manga, nil
}

func scanMangaWithStats(row rowScanner) (models.Manga, error) {
	manga, err := scanMangaPrefix(row)
	if err != nil {
		return manga, err
	}
	return manga, nil
}

func scanMangaPrefix(row rowScanner) (models.Manga, error) {
	var manga models.Manga
	var genresJSON string
	err := row.Scan(
		&manga.ID,
		&manga.Title,
		&manga.Author,
		&genresJSON,
		&manga.Status,
		&manga.TotalChapters,
		&manga.Description,
		&manga.CoverURL,
		&manga.SourceProvider,
		&manga.SourceURL,
		&manga.RightsStatus,
		&manga.PublicationYear,
		&manga.AverageRating,
		&manga.ReviewCount,
	)
	if err != nil {
		return manga, err
	}
	if err := json.Unmarshal([]byte(genresJSON), &manga.Genres); err != nil {
		return manga, err
	}
	manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
	return manga, nil
}

func (s *Server) attachReviewSummary(manga *models.Manga) {
	var average sql.NullFloat64
	var count int
	if err := s.DB.QueryRow(`SELECT AVG(rating), COUNT(*) FROM reviews WHERE manga_id = ?`, manga.ID).Scan(&average, &count); err != nil {
		return
	}
	if average.Valid {
		manga.AverageRating = average.Float64
	}
	manga.ReviewCount = count
}

func validateRegistration(req registerRequest) error {
	if len(req.Username) < 3 || len(req.Username) > 32 {
		return errors.New("username must be 3-32 characters")
	}
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(req.Username) {
		return errors.New("username may contain letters, numbers, and underscores only")
	}
	if !regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`).MatchString(req.Email) {
		return errors.New("invalid email format")
	}
	if len(req.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

func validateLibraryRequest(db *database.Store, req libraryRequest) error {
	statuses := map[string]bool{"reading": true, "completed": true, "plan-to-read": true, "on-hold": true, "dropped": true}
	if !statuses[req.Status] {
		return errors.New("status must be reading, completed, plan-to-read, on-hold, or dropped")
	}
	var total int
	if err := db.QueryRow(`SELECT total_chapters FROM manga WHERE id = ?`, req.MangaID).Scan(&total); err != nil {
		return errors.New("manga not found")
	}
	if req.CurrentChapter < 0 || (total > 0 && req.CurrentChapter > total) {
		return errors.New("chapter is outside the valid range")
	}
	return nil
}

func hasGenre(genres []string, target string) bool {
	for _, genre := range genres {
		if strings.ToLower(genre) == target {
			return true
		}
	}
	return false
}

func clampInt(raw string, minValue, maxValue int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func clampFloat(raw string, minValue, maxValue float64) float64 {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func normalizeMangaSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "rating", "reviews", "popular", "popularity", "chapters", "year", "newest", "latest":
		return strings.ToLower(strings.TrimSpace(sort))
	default:
		return "title"
	}
}

func mangaSearchOrderBy(sortMode string) string {
	switch sortMode {
	case "rating", "reviews":
		return "COALESCE(AVG(r.rating), 0) DESC, COUNT(r.id) DESC, lower(m.title) ASC"
	case "popular", "popularity":
		return "COUNT(r.id) DESC, COALESCE(AVG(r.rating), 0) DESC, lower(m.title) ASC"
	case "chapters":
		return "m.total_chapters DESC, lower(m.title) ASC"
	case "year", "newest", "latest":
		return "m.publication_year DESC, lower(m.title) ASC"
	default:
		return "lower(m.title) ASC"
	}
}
