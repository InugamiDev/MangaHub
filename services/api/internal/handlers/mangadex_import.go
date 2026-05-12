package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"

	"github.com/gin-gonic/gin"
)

const maxRemoteMangaDexPageBytes = 32 << 20

type mangaDexImportRequest struct {
	MangaID       string `json:"manga_id"`
	Language      string `json:"language"`
	Quality       string `json:"quality"`
	Limit         int    `json:"limit"`
	Offset        int    `json:"offset"`
	PublishStatus string `json:"publish_status"`
	RightsStatus  string `json:"rights_status"`
	ImportCover   *bool  `json:"import_cover"`
}

type mangaDexImportChapter struct {
	ID          string
	Title       string
	Number      float64
	PageCount   int
	External    bool
	ExternalURL string
}

type mangaDexImportFailure struct {
	ChapterID string `json:"chapter_id"`
	Message   string `json:"message"`
}

type mangaDexChapterListResponse struct {
	Result string `json:"result"`
	Data   []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Title              string `json:"title"`
			Chapter            string `json:"chapter"`
			Pages              int    `json:"pages"`
			TranslatedLanguage string `json:"translatedLanguage"`
			ExternalURL        string `json:"externalUrl"`
		} `json:"attributes"`
	} `json:"data"`
}

type mangaDexAtHomeResponse struct {
	Result  string `json:"result"`
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func (s *Server) importMangaDex(c *gin.Context) {
	mangaDexID := strings.ToLower(strings.TrimSpace(c.Param("id")))
	if !validMangaDexID(mangaDexID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "valid MangaDex manga id is required"})
		return
	}
	var req mangaDexImportRequest
	_ = c.ShouldBindJSON(&req)
	req = normalizeMangaDexImportRequest(req)

	sourceManga, err := fetchMangaDexMangaByID(mangaDexID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load MangaDex manga"})
		return
	}
	targetID := sanitizeRecordID(req.MangaID)
	if targetID == "" {
		targetID = sourceManga.ID
	}
	sourceManga.ID = targetID
	sourceManga.RightsStatus = req.RightsStatus
	sourceManga.SourceProvider = "MangaDex API + MangaHub import"

	coverStored := false
	if req.ImportCover == nil || *req.ImportCover {
		if storedCover, err := s.importRemoteAsset(c.Request.Context(), targetID, "covers", sourceManga.CoverURL); err == nil && storedCover != "" {
			sourceManga.CoverURL = storedCover
			sourceManga.CoverLargeURL = storedCover
			coverStored = true
		}
	}
	if err := database.UpsertManga(s.DB, []models.Manga{sourceManga}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to upsert imported manga"})
		return
	}

	chapters, err := fetchMangaDexImportChapters(mangaDexID, req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load MangaDex chapters"})
		return
	}

	imported := make([]models.Chapter, 0, len(chapters))
	failures := make([]mangaDexImportFailure, 0)
	for _, sourceChapter := range chapters {
		if sourceChapter.External {
			failures = append(failures, mangaDexImportFailure{ChapterID: sourceChapter.ID, Message: "external chapter has no MangaDex@Home pages"})
			continue
		}
		pageURLs, err := s.importMangaDexChapterPages(c.Request.Context(), targetID, sourceChapter.ID, req.Quality)
		if err != nil {
			failures = append(failures, mangaDexImportFailure{ChapterID: sourceChapter.ID, Message: err.Error()})
			continue
		}
		chapterID := "mangadex-" + sourceChapter.ID
		chapter := models.Chapter{
			ID:            chapterID,
			MangaID:       targetID,
			Title:         firstNonEmpty(sourceChapter.Title, defaultChapterTitle(sourceChapter.Number)),
			ChapterNumber: sourceChapter.Number,
			PageURLs:      pageURLs,
			PageCount:     len(pageURLs),
			PublishStatus: req.PublishStatus,
			UpdatedAt:     time.Now().UTC(),
		}
		if err := s.upsertImportedChapter(chapter); err != nil {
			failures = append(failures, mangaDexImportFailure{ChapterID: sourceChapter.ID, Message: "failed to save chapter record"})
			continue
		}
		imported = append(imported, chapter)
	}

	_ = s.recalculateTotalChapters(targetID)
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, gin.H{
		"status":         "imported",
		"manga_id":       targetID,
		"mangadex_id":    mangaDexID,
		"rights_status":  req.RightsStatus,
		"storage":        "chapter-storage-adapter",
		"cover_stored":   coverStored,
		"imported_count": len(imported),
		"failure_count":  len(failures),
		"chapters":       imported,
		"failures":       failures,
	})
}

func normalizeMangaDexImportRequest(req mangaDexImportRequest) mangaDexImportRequest {
	req.Language = strings.ToLower(strings.TrimSpace(req.Language))
	if req.Language == "" {
		req.Language = "en"
	}
	req.Quality = strings.ToLower(strings.TrimSpace(req.Quality))
	if req.Quality == "" {
		req.Quality = "data-saver"
	}
	if req.Quality != "data" && req.Quality != "data-saver" {
		req.Quality = "data-saver"
	}
	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 50 {
		req.Limit = 50
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	req.PublishStatus = strings.TrimSpace(req.PublishStatus)
	if req.PublishStatus == "" {
		req.PublishStatus = "published"
	}
	req.RightsStatus = strings.TrimSpace(req.RightsStatus)
	if req.RightsStatus == "" || req.RightsStatus == "metadata-only" {
		req.RightsStatus = "licensed-import"
	}
	return req
}

func fetchMangaDexImportChapters(mangaDexID string, req mangaDexImportRequest) ([]mangaDexImportChapter, error) {
	u, err := url.Parse(mangaDexAPIBase + "/manga/" + mangaDexID + "/feed")
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("limit", strconv.Itoa(req.Limit))
	query.Set("offset", strconv.Itoa(req.Offset))
	query.Set("order[chapter]", "asc")
	query.Set("includeExternalUrl", "0")
	if req.Language != "" {
		query.Add("translatedLanguage[]", req.Language)
	}
	u.RawQuery = query.Encode()

	var payload mangaDexChapterListResponse
	if err := getMangaDexJSON(u.String(), &payload); err != nil {
		return nil, err
	}
	if strings.ToLower(payload.Result) != "ok" {
		return nil, errors.New("mangadex returned non-ok chapter feed")
	}
	chapters := make([]mangaDexImportChapter, 0, len(payload.Data))
	for index, item := range payload.Data {
		if item.Type != "" && item.Type != "chapter" {
			continue
		}
		number := float64(index + 1 + req.Offset)
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(item.Attributes.Chapter), 64); err == nil {
			number = parsed
		}
		chapters = append(chapters, mangaDexImportChapter{
			ID:          item.ID,
			Title:       strings.TrimSpace(item.Attributes.Title),
			Number:      number,
			PageCount:   item.Attributes.Pages,
			External:    strings.TrimSpace(item.Attributes.ExternalURL) != "",
			ExternalURL: item.Attributes.ExternalURL,
		})
	}
	return chapters, nil
}

func fetchMangaDexAtHome(chapterID string) (mangaDexAtHomeResponse, error) {
	u, err := url.Parse(mangaDexAPIBase + "/at-home/server/" + chapterID)
	if err != nil {
		return mangaDexAtHomeResponse{}, err
	}
	query := u.Query()
	query.Set("forcePort443", "true")
	u.RawQuery = query.Encode()

	var payload mangaDexAtHomeResponse
	if err := getMangaDexJSON(u.String(), &payload); err != nil {
		return payload, err
	}
	if strings.ToLower(payload.Result) != "ok" || payload.BaseURL == "" || payload.Chapter.Hash == "" {
		return payload, errors.New("mangadex at-home response is incomplete")
	}
	return payload, nil
}

func (s *Server) importMangaDexChapterPages(ctx context.Context, mangaID, sourceChapterID, quality string) ([]string, error) {
	atHome, err := fetchMangaDexAtHome(sourceChapterID)
	if err != nil {
		return nil, err
	}
	folder := "data"
	files := atHome.Chapter.Data
	if quality == "data-saver" && len(atHome.Chapter.DataSaver) > 0 {
		folder = "data-saver"
		files = atHome.Chapter.DataSaver
	}
	if len(files) == 0 {
		return nil, errors.New("chapter has no importable page files")
	}
	targetChapterID := "mangadex-" + sourceChapterID
	pageURLs := make([]string, 0, len(files))
	for _, fileName := range files {
		remoteURL := strings.TrimRight(atHome.BaseURL, "/") + "/" + folder + "/" + atHome.Chapter.Hash + "/" + fileName
		storedURL, err := s.importRemoteAsset(ctx, mangaID, targetChapterID, remoteURL)
		if err != nil {
			return nil, err
		}
		pageURLs = append(pageURLs, storedURL)
	}
	return pageURLs, nil
}

func (s *Server) importRemoteAsset(ctx context.Context, mangaID, chapterID, remoteURL string) (string, error) {
	filename, contentType, body, err := downloadRemoteImage(ctx, remoteURL)
	if err != nil {
		return "", err
	}
	return s.ChapterStorage.SaveBytes(ctx, mangaID, chapterID, filename, contentType, body)
}

func downloadRemoteImage(ctx context.Context, remoteURL string) (string, string, []byte, error) {
	parsed, err := url.Parse(remoteURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", "", nil, errors.New("remote image URL is invalid")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, remoteURL, nil)
	if err != nil {
		return "", "", nil, errors.New("failed to create image request")
	}
	req.Header.Set("Accept", "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8")
	req.Header.Set("User-Agent", "MangaHub/1.0 licensed import")
	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", nil, errors.New("failed to download remote image")
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", "", nil, fmt.Errorf("remote image returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxRemoteMangaDexPageBytes+1))
	if err != nil {
		return "", "", nil, errors.New("failed to read remote image")
	}
	if len(body) > maxRemoteMangaDexPageBytes {
		return "", "", nil, errors.New("remote image exceeds import size limit")
	}
	contentType := strings.Split(resp.Header.Get("Content-Type"), ";")[0]
	contentType = strings.TrimSpace(contentType)
	if contentType != "" && !strings.HasPrefix(contentType, "image/") {
		return "", "", nil, errors.New("remote asset is not an image")
	}
	if isMangaDexNoticeImage(parsed.Host, contentType, body) {
		return "", "", nil, errors.New("mangadex returned a notice image instead of a readable chapter page")
	}
	filename := filepath.Base(parsed.Path)
	if filepath.Ext(filename) == "" && contentType != "" {
		if exts, err := mime.ExtensionsByType(contentType); err == nil && len(exts) > 0 {
			filename += exts[0]
		}
	}
	if filepath.Ext(filename) == "" {
		return "", "", nil, errors.New("remote image has no supported extension")
	}
	return filename, contentType, body, nil
}

func isMangaDexNoticeImage(host, contentType string, body []byte) bool {
	if !strings.Contains(strings.ToLower(host), "mangadex") {
		return false
	}
	if contentType != "" && !strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return false
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		return false
	}
	return config.Width == 600 && config.Height == 650
}

func (s *Server) upsertImportedChapter(chapter models.Chapter) error {
	if err := s.insertChapter(chapter); err == nil {
		return nil
	} else if !isUniqueConflict(err) {
		return err
	}
	return s.updateChapterRecord(chapter)
}

func isUniqueConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "unique") || strings.Contains(message, "duplicate")
}
