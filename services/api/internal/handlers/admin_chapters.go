package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mangahub/services/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type mangaMutationRequest struct {
	ID             *string   `json:"id"`
	Title          *string   `json:"title"`
	Author         *string   `json:"author"`
	Genres         *[]string `json:"genres"`
	Status         *string   `json:"status"`
	TotalChapters  *int      `json:"total_chapters"`
	Description    *string   `json:"description"`
	CoverURL       *string   `json:"cover_url"`
	SourceProvider *string   `json:"source_provider"`
	SourceURL      *string   `json:"source_url"`
	RightsStatus   *string   `json:"rights_status"`
}

type chapterMutationRequest struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	ChapterNumber *float64 `json:"chapter_number"`
	Number        *float64 `json:"number"`
	PageURLs      []string `json:"page_urls"`
	Pages         []string `json:"pages"`
	PageCount     *int     `json:"page_count"`
	PublishStatus string   `json:"publish_status"`
}

func (s *Server) createManga(c *gin.Context) {
	var req mangaMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}

	title := stringFromPtr(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	mangaID := sanitizeRecordID(stringFromPtr(req.ID))
	if mangaID == "" {
		mangaID = slugify(title)
	}
	if mangaID == "" {
		mangaID = "manga-" + uuid.NewString()
	}

	manga := models.Manga{
		ID:             mangaID,
		Title:          title,
		Author:         firstNonEmpty(stringFromPtr(req.Author), "Unknown"),
		Genres:         normalizeGenres(sliceFromPtr(req.Genres)),
		Status:         firstNonEmpty(stringFromPtr(req.Status), "ongoing"),
		TotalChapters:  intFromPtr(req.TotalChapters),
		Description:    stringFromPtr(req.Description),
		CoverURL:       models.SanitizeCoverURL(stringFromPtr(req.CoverURL)),
		SourceProvider: firstNonEmpty(stringFromPtr(req.SourceProvider), "MangaHub admin"),
		SourceURL:      stringFromPtr(req.SourceURL),
		RightsStatus:   firstNonEmpty(stringFromPtr(req.RightsStatus), "admin-managed"),
	}
	if err := validateMangaMutation(manga); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	genres, err := json.Marshal(manga.Genres)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode genres"})
		return
	}
	_, err = s.DB.Exec(`
INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		manga.ID, manga.Title, manga.Author, string(genres), manga.Status, manga.TotalChapters, manga.Description, manga.CoverURL, manga.SourceProvider, manga.SourceURL, manga.RightsStatus)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "manga already exists or could not be created"})
		return
	}
	s.invalidateCatalogCache()
	c.JSON(http.StatusCreated, manga)
}

func (s *Server) updateManga(c *gin.Context) {
	manga, err := s.findManga(c.Param("id"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load manga"})
		return
	}

	var req mangaMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	applyMangaMutation(&manga, req)
	if err := validateMangaMutation(manga); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	genres, err := json.Marshal(manga.Genres)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to encode genres"})
		return
	}
	_, err = s.DB.Exec(`
UPDATE manga
SET title = ?, author = ?, genres = ?, status = ?, total_chapters = ?, description = ?,
    cover_url = ?, source_provider = ?, source_url = ?, rights_status = ?
WHERE id = ?`,
		manga.Title, manga.Author, string(genres), manga.Status, manga.TotalChapters, manga.Description,
		manga.CoverURL, manga.SourceProvider, manga.SourceURL, manga.RightsStatus, manga.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update manga"})
		return
	}
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, manga)
}

func (s *Server) deleteManga(c *gin.Context) {
	result, err := s.DB.Exec(`DELETE FROM manga WHERE id = ?`, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete manga"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) listChapters(c *gin.Context) {
	if !s.mangaExists(c.Param("id")) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	rows, err := s.DB.Query(`
SELECT id, manga_id, title, chapter_number, page_urls, page_count, publish_status, updated_at
FROM chapters
WHERE manga_id = ?
ORDER BY chapter_number ASC, updated_at ASC`, c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chapters"})
		return
	}
	defer rows.Close()

	chapters := make([]models.Chapter, 0)
	for rows.Next() {
		chapter, err := scanChapter(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid chapter record"})
			return
		}
		chapters = append(chapters, chapter)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chapters"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"chapters": chapters, "count": len(chapters)})
}

func (s *Server) getChapter(c *gin.Context) {
	chapter, err := s.findChapter(c.Param("id"), c.Param("chapterId"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "chapter not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chapter"})
		return
	}
	c.JSON(http.StatusOK, chapter)
}

func (s *Server) createChapter(c *gin.Context) {
	mangaID := c.Param("id")
	if !s.mangaExists(mangaID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}

	req, uploadedPages, err := s.parseChapterMutation(c, mangaID, "")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	chapterID := sanitizeRecordID(req.ID)
	if chapterID == "" {
		chapterID = "chapter-" + uuid.NewString()
	}
	pageURLs := append(normalizePageURLs(append(req.PageURLs, req.Pages...)), uploadedPages...)
	chapterNumber := chapterNumberFromRequest(req, 0)
	chapter := models.Chapter{
		ID:            chapterID,
		MangaID:       mangaID,
		Title:         firstNonEmpty(strings.TrimSpace(req.Title), defaultChapterTitle(chapterNumber)),
		ChapterNumber: chapterNumber,
		PageURLs:      pageURLs,
		PageCount:     len(pageURLs),
		PublishStatus: firstNonEmpty(strings.TrimSpace(req.PublishStatus), "draft"),
		UpdatedAt:     time.Now().UTC(),
	}
	if req.PageCount != nil {
		chapter.PageCount = *req.PageCount
	}
	if err := validateChapterMutation(chapter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.insertChapter(chapter); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "chapter already exists or could not be created"})
		return
	}
	_ = s.recalculateTotalChapters(mangaID)
	s.invalidateCatalogCache()
	c.JSON(http.StatusCreated, chapter)
}

func (s *Server) updateChapter(c *gin.Context) {
	mangaID := c.Param("id")
	existing, err := s.findChapter(mangaID, c.Param("chapterId"))
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "chapter not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chapter"})
		return
	}

	req, uploadedPages, err := s.parseChapterMutation(c, mangaID, existing.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Title) != "" {
		existing.Title = strings.TrimSpace(req.Title)
	}
	if req.ChapterNumber != nil || req.Number != nil {
		existing.ChapterNumber = chapterNumberFromRequest(req, existing.ChapterNumber)
	}
	if req.PageURLs != nil || req.Pages != nil {
		existing.PageURLs = normalizePageURLs(append(req.PageURLs, req.Pages...))
	}
	if len(uploadedPages) > 0 {
		existing.PageURLs = append(existing.PageURLs, uploadedPages...)
	}
	if req.PageCount != nil {
		existing.PageCount = *req.PageCount
	} else {
		existing.PageCount = len(existing.PageURLs)
	}
	if strings.TrimSpace(req.PublishStatus) != "" {
		existing.PublishStatus = strings.TrimSpace(req.PublishStatus)
	}
	existing.UpdatedAt = time.Now().UTC()
	if err := validateChapterMutation(existing); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.updateChapterRecord(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update chapter"})
		return
	}
	_ = s.recalculateTotalChapters(mangaID)
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, existing)
}

func (s *Server) deleteChapter(c *gin.Context) {
	result, err := s.DB.Exec(`DELETE FROM chapters WHERE manga_id = ? AND id = ?`, c.Param("id"), c.Param("chapterId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete chapter"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "chapter not found"})
		return
	}
	_ = s.recalculateTotalChapters(c.Param("id"))
	s.invalidateCatalogCache()
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) deleteLibrary(c *gin.Context) {
	userID := c.GetString("user_id")
	result, err := s.DB.Exec(`DELETE FROM user_progress WHERE user_id = ? AND manga_id = ?`, userID, c.Param("mangaID"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to remove manga from library"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga is not in your library"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "removed"})
}

func (s *Server) findManga(id string) (models.Manga, error) {
	row := s.DB.QueryRow(`
SELECT id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status
FROM manga WHERE id = ?`, id)
	return scanManga(row)
}

func (s *Server) mangaExists(id string) bool {
	var exists int
	return s.DB.QueryRow(`SELECT 1 FROM manga WHERE id = ?`, id).Scan(&exists) == nil
}

func (s *Server) findChapter(mangaID, chapterID string) (models.Chapter, error) {
	row := s.DB.QueryRow(`
SELECT id, manga_id, title, chapter_number, page_urls, page_count, publish_status, updated_at
FROM chapters
WHERE manga_id = ? AND id = ?`, mangaID, chapterID)
	chapter, err := scanChapter(row)
	if err == nil || !errors.Is(err, sql.ErrNoRows) {
		return chapter, err
	}
	number, parseErr := strconv.ParseFloat(chapterID, 64)
	if parseErr != nil {
		return chapter, err
	}
	row = s.DB.QueryRow(`
SELECT id, manga_id, title, chapter_number, page_urls, page_count, publish_status, updated_at
FROM chapters
WHERE manga_id = ? AND chapter_number = ?
ORDER BY updated_at DESC
LIMIT 1`, mangaID, number)
	chapter, err = scanChapter(row)
	return chapter, err
}

func (s *Server) insertChapter(chapter models.Chapter) error {
	pages, err := json.Marshal(chapter.PageURLs)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`
INSERT INTO chapters (id, manga_id, title, chapter_number, page_urls, page_count, publish_status, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		chapter.ID, chapter.MangaID, chapter.Title, chapter.ChapterNumber, string(pages), chapter.PageCount, chapter.PublishStatus)
	return err
}

func (s *Server) updateChapterRecord(chapter models.Chapter) error {
	pages, err := json.Marshal(chapter.PageURLs)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec(`
UPDATE chapters
SET title = ?, chapter_number = ?, page_urls = ?, page_count = ?, publish_status = ?, updated_at = CURRENT_TIMESTAMP
WHERE manga_id = ? AND id = ?`,
		chapter.Title, chapter.ChapterNumber, string(pages), chapter.PageCount, chapter.PublishStatus, chapter.MangaID, chapter.ID)
	return err
}

func (s *Server) recalculateTotalChapters(mangaID string) error {
	_, err := s.DB.Exec(`
UPDATE manga
SET total_chapters = (SELECT COUNT(*) FROM chapters WHERE manga_id = ?)
WHERE id = ?`, mangaID, mangaID)
	return err
}

func (s *Server) parseChapterMutation(c *gin.Context, mangaID, chapterID string) (chapterMutationRequest, []string, error) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		return s.parseMultipartChapterMutation(c, mangaID, chapterID)
	}
	var req chapterMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return req, nil, errors.New("invalid JSON body")
	}
	return req, nil, nil
}

func (s *Server) parseMultipartChapterMutation(c *gin.Context, mangaID, chapterID string) (chapterMutationRequest, []string, error) {
	var req chapterMutationRequest
	req.ID = strings.TrimSpace(c.PostForm("id"))
	req.Title = strings.TrimSpace(c.PostForm("title"))
	req.PublishStatus = strings.TrimSpace(c.PostForm("publish_status"))
	if raw := strings.TrimSpace(firstNonEmpty(c.PostForm("chapter_number"), c.PostForm("number"))); raw != "" {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return req, nil, errors.New("chapter_number must be numeric")
		}
		req.ChapterNumber = &value
	}
	if raw := strings.TrimSpace(c.PostForm("page_count")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return req, nil, errors.New("page_count must be an integer")
		}
		req.PageCount = &value
	}
	req.PageURLs = parseFormPageURLs(c)

	form, err := c.MultipartForm()
	if err != nil {
		return req, nil, err
	}
	targetChapterID := firstNonEmpty(chapterID, sanitizeRecordID(req.ID))
	if targetChapterID == "" && len(form.File) > 0 {
		targetChapterID = "chapter-" + uuid.NewString()
		req.ID = targetChapterID
	}
	uploads, err := s.saveUploadedChapterPages(c.Request.Context(), mangaID, targetChapterID, form.File)
	if err != nil {
		return req, nil, err
	}
	return req, uploads, nil
}

func (s *Server) saveUploadedChapterPages(ctx context.Context, mangaID, chapterID string, files map[string][]*multipart.FileHeader) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	if chapterID == "" {
		return nil, errors.New("chapter id is required for page uploads")
	}
	keys := make([]string, 0, len(files))
	for key := range files {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	pageURLs := make([]string, 0)
	for _, key := range keys {
		for _, header := range files[key] {
			ext := strings.ToLower(filepath.Ext(header.Filename))
			if !allowedPageExtension(ext) {
				return nil, fmt.Errorf("unsupported page file type %q", ext)
			}
			pageURL, err := s.ChapterStorage.Save(ctx, mangaID, chapterID, header)
			if err != nil {
				return nil, err
			}
			pageURLs = append(pageURLs, pageURL)
		}
	}
	return pageURLs, nil
}

func scanChapter(row rowScanner) (models.Chapter, error) {
	var chapter models.Chapter
	var pagesJSON string
	var updated any
	err := row.Scan(&chapter.ID, &chapter.MangaID, &chapter.Title, &chapter.ChapterNumber, &pagesJSON, &chapter.PageCount, &chapter.PublishStatus, &updated)
	if err != nil {
		return chapter, err
	}
	if err := json.Unmarshal([]byte(pagesJSON), &chapter.PageURLs); err != nil {
		return chapter, err
	}
	chapter.PageURLs = normalizePageURLs(chapter.PageURLs)
	if chapter.PageCount < 0 {
		chapter.PageCount = len(chapter.PageURLs)
	}
	chapter.UpdatedAt = parseSQLTime(updated)
	return chapter, nil
}

func applyMangaMutation(manga *models.Manga, req mangaMutationRequest) {
	if req.Title != nil {
		manga.Title = strings.TrimSpace(*req.Title)
	}
	if req.Author != nil {
		manga.Author = strings.TrimSpace(*req.Author)
	}
	if req.Genres != nil {
		manga.Genres = normalizeGenres(*req.Genres)
	}
	if req.Status != nil {
		manga.Status = strings.TrimSpace(*req.Status)
	}
	if req.TotalChapters != nil {
		manga.TotalChapters = *req.TotalChapters
	}
	if req.Description != nil {
		manga.Description = strings.TrimSpace(*req.Description)
	}
	if req.CoverURL != nil {
		manga.CoverURL = models.SanitizeCoverURL(*req.CoverURL)
	}
	if req.SourceProvider != nil {
		manga.SourceProvider = strings.TrimSpace(*req.SourceProvider)
	}
	if req.SourceURL != nil {
		manga.SourceURL = strings.TrimSpace(*req.SourceURL)
	}
	if req.RightsStatus != nil {
		manga.RightsStatus = strings.TrimSpace(*req.RightsStatus)
	}
}

func validateMangaMutation(manga models.Manga) error {
	if strings.TrimSpace(manga.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(manga.Title) == "" {
		return errors.New("title is required")
	}
	if strings.TrimSpace(manga.Author) == "" {
		return errors.New("author is required")
	}
	if manga.TotalChapters < 0 {
		return errors.New("total_chapters cannot be negative")
	}
	return nil
}

func validateChapterMutation(chapter models.Chapter) error {
	if strings.TrimSpace(chapter.ID) == "" {
		return errors.New("id is required")
	}
	if strings.TrimSpace(chapter.MangaID) == "" {
		return errors.New("manga_id is required")
	}
	if strings.TrimSpace(chapter.Title) == "" {
		return errors.New("title is required")
	}
	if chapter.ChapterNumber < 0 {
		return errors.New("chapter_number cannot be negative")
	}
	if chapter.PageCount < 0 {
		return errors.New("page_count cannot be negative")
	}
	if strings.TrimSpace(chapter.PublishStatus) == "" {
		return errors.New("publish_status is required")
	}
	return nil
}

func normalizeGenres(values []string) []string {
	genres := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		genre := strings.TrimSpace(value)
		key := strings.ToLower(genre)
		if genre != "" && !seen[key] {
			genres = append(genres, genre)
			seen[key] = true
		}
	}
	return genres
}

func normalizePageURLs(values []string) []string {
	pageURLs := make([]string, 0, len(values))
	for _, value := range values {
		if sanitized := sanitizePageURL(value); sanitized != "" {
			pageURLs = append(pageURLs, sanitized)
		}
	}
	return pageURLs
}

func sanitizePageURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "/media/chapters/") {
		return trimmed
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return trimmed
}

func parseFormPageURLs(c *gin.Context) []string {
	values := c.PostFormArray("page_urls")
	values = append(values, c.PostFormArray("pages")...)
	for _, key := range []string{"page_urls", "pages"} {
		raw := strings.TrimSpace(c.PostForm(key))
		if strings.HasPrefix(raw, "[") {
			var decoded []string
			if json.Unmarshal([]byte(raw), &decoded) == nil {
				values = append(values, decoded...)
			}
		}
	}
	return normalizePageURLs(values)
}

func chapterNumberFromRequest(req chapterMutationRequest, fallback float64) float64 {
	if req.ChapterNumber != nil {
		return *req.ChapterNumber
	}
	if req.Number != nil {
		return *req.Number
	}
	return fallback
}

func defaultChapterTitle(number float64) string {
	if number == float64(int64(number)) {
		return "Chapter " + strconv.FormatInt(int64(number), 10)
	}
	return "Chapter " + strconv.FormatFloat(number, 'f', -1, 64)
}

func stringFromPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sliceFromPtr(value *[]string) []string {
	if value == nil {
		return nil
	}
	return *value
}

func intFromPtr(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func slugify(value string) string {
	slug := strings.ToLower(strings.TrimSpace(value))
	slug = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 80 {
		slug = strings.Trim(slug[:80], "-")
	}
	return slug
}

func sanitizeRecordID(value string) string {
	id := strings.TrimSpace(value)
	id = regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(id, "-")
	id = strings.Trim(id, "-")
	if len(id) > 120 {
		id = id[:120]
	}
	return id
}

func sanitizePathPart(value string) string {
	part := sanitizeRecordID(value)
	if part == "" {
		return "unknown"
	}
	return part
}

func allowedPageExtension(ext string) bool {
	switch ext {
	case ".jpg", ".jpeg", ".png", ".webp", ".gif":
		return true
	default:
		return false
	}
}
