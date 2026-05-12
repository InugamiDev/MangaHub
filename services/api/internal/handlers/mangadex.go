package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"
)

const (
	mangaDexAPIBase     = "https://api.mangadex.org"
	mangaDexCoverCDN    = "https://uploads.mangadex.org/covers"
	mangaDexTitleOrigin = "https://mangadex.org/title"
)

var mangaDexCache sync.Map

var mangaDexStatusAliases = map[string]string{
	"":          "",
	"all":       "",
	"any":       "",
	"completed": "completed",
	"complete":  "completed",
	"finished":  "completed",
	"ongoing":   "ongoing",
	"releasing": "ongoing",
	"current":   "ongoing",
	"hiatus":    "hiatus",
	"cancelled": "cancelled",
	"canceled":  "cancelled",
}

type mangaDexListResponse struct {
	Result string           `json:"result"`
	Total  int              `json:"total"`
	Data   []mangaDexEntity `json:"data"`
}

type mangaDexEntityResponse struct {
	Result string         `json:"result"`
	Data   mangaDexEntity `json:"data"`
}

type mangaDexEntity struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Attributes    mangaDexAttributes     `json:"attributes"`
	Relationships []mangaDexRelationship `json:"relationships"`
}

type mangaDexAttributes struct {
	Title         map[string]string      `json:"title"`
	AltTitles     []map[string]string    `json:"altTitles"`
	Description   map[string]string      `json:"description"`
	Status        string                 `json:"status"`
	LastChapter   string                 `json:"lastChapter"`
	Year          int                    `json:"year"`
	ContentRating string                 `json:"contentRating"`
	Tags          []mangaDexTagContainer `json:"tags"`
}

type mangaDexTagContainer struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name  map[string]string `json:"name"`
		Group string            `json:"group"`
	} `json:"attributes"`
}

type mangaDexRelationship struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Attributes struct {
		Name     string `json:"name"`
		FileName string `json:"fileName"`
	} `json:"attributes"`
}

func fetchMangaDexMangaWithStatus(search, status string, limit int) ([]models.Manga, error) {
	u, err := url.Parse(mangaDexAPIBase + "/manga")
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("limit", strconv.Itoa(limit))
	query.Add("includes[]", "cover_art")
	query.Add("includes[]", "author")
	query.Add("includes[]", "artist")
	query.Add("contentRating[]", "safe")
	query.Add("contentRating[]", "suggestive")
	if strings.TrimSpace(search) != "" {
		query.Set("title", strings.TrimSpace(search))
		query.Set("order[relevance]", "desc")
	} else {
		query.Set("order[followedCount]", "desc")
	}
	if strings.TrimSpace(status) != "" {
		query.Add("status[]", status)
	}
	u.RawQuery = query.Encode()

	var payload mangaDexListResponse
	if err := getMangaDexJSON(u.String(), &payload); err != nil {
		return nil, err
	}
	if strings.ToLower(payload.Result) != "ok" {
		return nil, errors.New("mangadex returned non-ok result")
	}
	results := make([]models.Manga, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Type != "" && item.Type != "manga" {
			continue
		}
		results = append(results, mapMangaDexEntity(item))
	}
	return results, nil
}

func fetchMangaDexMangaByID(id string) (models.Manga, error) {
	if !validMangaDexID(id) {
		return models.Manga{}, errors.New("invalid MangaDex id")
	}
	u, err := url.Parse(mangaDexAPIBase + "/manga/" + id)
	if err != nil {
		return models.Manga{}, err
	}
	query := u.Query()
	query.Add("includes[]", "cover_art")
	query.Add("includes[]", "author")
	query.Add("includes[]", "artist")
	u.RawQuery = query.Encode()

	var payload mangaDexEntityResponse
	if err := getMangaDexJSON(u.String(), &payload); err != nil {
		return models.Manga{}, err
	}
	if strings.ToLower(payload.Result) != "ok" || payload.Data.ID == "" {
		return models.Manga{}, errors.New("mangadex manga not found")
	}
	return mapMangaDexEntity(payload.Data), nil
}

func getMangaDexJSON(rawURL string, target any) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "MangaHub/1.0 metadata adapter")
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("mangadex returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func SyncMangaDexCatalog(db *database.Store, search string, statuses []string, limit int) (map[string]int, int, error) {
	return syncMangaDexCatalog(db, search, statuses, limit)
}

func syncMangaDexCatalog(db *database.Store, search string, statuses []string, limit int) (map[string]int, int, error) {
	if db == nil {
		return nil, 0, errors.New("database is required")
	}
	normalizedStatuses, err := normalizeMangaDexStatusBuckets(statuses)
	if err != nil {
		return nil, 0, err
	}
	stats := make(map[string]int, len(normalizedStatuses))
	entries := make([]models.Manga, 0, len(normalizedStatuses)*limit)
	seen := make(map[string]bool)
	for _, status := range normalizedStatuses {
		results, err := fetchMangaDexMangaWithStatus(search, status, limit)
		if err != nil {
			return stats, len(entries), err
		}
		key := mangaDexStatusResponseKey(status)
		for _, manga := range results {
			if manga.ID == "" || seen[manga.ID] {
				continue
			}
			seen[manga.ID] = true
			manga.SourceProvider = "MangaDex API"
			manga.RightsStatus = "metadata-only"
			manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
			manga.CoverLargeURL = models.SanitizeCoverURL(firstNonEmpty(manga.CoverLargeURL, manga.CoverURL))
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

func mapMangaDexEntity(item mangaDexEntity) models.Manga {
	title := localizedMangaDexText(item.Attributes.Title, "Untitled manga")
	author := firstMangaDexRelationshipName(item.Relationships, "author")
	if author == "" {
		author = firstMangaDexRelationshipName(item.Relationships, "artist")
	}
	if author == "" {
		author = "Unknown"
	}
	genres, tags := mapMangaDexTags(item.Attributes.Tags)
	chapterCount := parseMangaDexChapterCount(item.Attributes.LastChapter)
	coverURL := mangaDexCoverURL(item.ID, firstMangaDexCoverFile(item.Relationships))
	sourceURL := mangaDexTitleOrigin + "/" + item.ID
	return models.Manga{
		ID:             "mangadex-" + item.ID,
		Title:          title,
		Author:         author,
		Genres:         genres,
		Status:         normalizeMangaDexStatus(item.Attributes.Status),
		TotalChapters:  chapterCount,
		Description:    cleanSourceDescription(localizedMangaDexText(item.Attributes.Description, "")),
		CoverURL:       coverURL,
		CoverLargeURL:  coverURL,
		SourceProvider: "MangaDex API",
		SourceURL:      sourceURL,
		RightsStatus:   "metadata-only",
		Format:         "manga",
		Tags:           tags,
		ExternalLinks: []models.MangaExternalLink{
			{
				ID:   stableMangaDexInt(item.ID),
				Site: "MangaDex",
				URL:  sourceURL,
				Type: "source",
			},
		},
	}
}

func cachedMangaDexManga(id string) (models.Manga, error) {
	if cached, ok := mangaDexCache.Load(id); ok {
		if manga, ok := cached.(models.Manga); ok {
			return manga, nil
		}
	}
	manga, err := fetchMangaDexMangaByID(id)
	if err != nil {
		return models.Manga{}, err
	}
	mangaDexCache.Store(id, manga)
	return manga, nil
}

func mergeMangaDexSource(base, source models.Manga) models.Manga {
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

func localizedMangaDexText(values map[string]string, fallback string) string {
	for _, key := range []string{"en", "ja-ro", "ja", "ko", "zh", "fr", "de", "es", "it", "pt-br"} {
		if value := strings.TrimSpace(values[key]); value != "" {
			return value
		}
	}
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func firstMangaDexRelationshipName(relationships []mangaDexRelationship, relationshipType string) string {
	for _, relationship := range relationships {
		if relationship.Type == relationshipType && strings.TrimSpace(relationship.Attributes.Name) != "" {
			return strings.TrimSpace(relationship.Attributes.Name)
		}
	}
	return ""
}

func firstMangaDexCoverFile(relationships []mangaDexRelationship) string {
	for _, relationship := range relationships {
		if relationship.Type == "cover_art" && strings.TrimSpace(relationship.Attributes.FileName) != "" {
			return strings.TrimSpace(relationship.Attributes.FileName)
		}
	}
	return ""
}

func mangaDexCoverURL(mangaID, fileName string) string {
	if !validMangaDexID(mangaID) || strings.TrimSpace(fileName) == "" {
		return ""
	}
	return models.SanitizeCoverURL(mangaDexCoverCDN + "/" + mangaID + "/" + strings.TrimSpace(fileName))
}

func mapMangaDexTags(values []mangaDexTagContainer) ([]string, []models.MangaTag) {
	genres := make([]string, 0, len(values))
	tags := make([]models.MangaTag, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		name := localizedMangaDexText(value.Attributes.Name, "")
		if name == "" {
			continue
		}
		key := strings.ToLower(name)
		if !seen[key] && (value.Attributes.Group == "" || value.Attributes.Group == "genre" || len(genres) < 6) {
			genres = append(genres, name)
			seen[key] = true
		}
		tags = append(tags, models.MangaTag{
			ID:          stableMangaDexInt(value.ID),
			Name:        name,
			Description: value.Attributes.Group,
			Rank:        0,
		})
		if len(tags) >= 16 {
			break
		}
	}
	if len(genres) == 0 {
		genres = []string{"Manga"}
	}
	if len(genres) > 10 {
		genres = genres[:10]
	}
	return genres, tags
}

func parseMangaDexChapterCount(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	number, err := strconv.ParseFloat(trimmed, 64)
	if err != nil || number < 0 {
		return 0
	}
	return int(number)
}

func validMangaDexID(id string) bool {
	return regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`).MatchString(strings.TrimSpace(id))
}

func extractMangaDexMangaID(value string) string {
	matches := regexp.MustCompile(`(?i)mangadex\.org/title/([0-9a-f-]{36})`).FindStringSubmatch(value)
	if len(matches) < 2 || !validMangaDexID(matches[1]) {
		return ""
	}
	return strings.ToLower(matches[1])
}

func normalizeMangaDexRequestStatus(status string) (string, bool) {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(status), "_", "-"))
	if value, ok := mangaDexStatusAliases[key]; ok {
		return value, true
	}
	switch key {
	case "completed", "ongoing", "hiatus", "cancelled":
		return key, true
	default:
		return "", false
	}
}

func normalizeMangaDexStatusBuckets(statuses []string) ([]string, error) {
	if len(statuses) == 0 {
		statuses = []string{"completed", "ongoing", "hiatus", "cancelled"}
	}
	normalized := make([]string, 0, len(statuses))
	seen := make(map[string]bool)
	for _, status := range statuses {
		value, ok := normalizeMangaDexRequestStatus(status)
		if !ok {
			return nil, fmt.Errorf("invalid MangaDex status %q", status)
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
		return []string{"completed", "ongoing", "hiatus", "cancelled"}, nil
	}
	return normalized, nil
}

func normalizeMangaDexStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "completed":
		return "completed"
	case "ongoing":
		return "ongoing"
	case "hiatus":
		return "hiatus"
	case "cancelled":
		return "cancelled"
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func mangaDexStatusResponseKey(status string) string {
	if strings.TrimSpace(status) == "" {
		return "all"
	}
	return normalizeMangaDexStatus(status)
}

func stableMangaDexInt(value string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(value))
	return int(hash.Sum32() & 0x7fffffff)
}
