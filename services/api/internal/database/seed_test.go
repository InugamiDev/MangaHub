package database

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"mangahub/services/api/internal/models"
)

func TestSeedSourceURLsPointToTitlePages(t *testing.T) {
	payload, err := os.ReadFile("../../data/manga.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var entries []models.Manga
	if err := json.Unmarshal(payload, &entries); err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.SourceURL, "/search/") {
			t.Fatalf("%s source_url points to search page: %s", entry.ID, entry.SourceURL)
		}
		if strings.Contains(entry.SourceURL, "?search=") {
			t.Fatalf("%s source_url contains search query: %s", entry.ID, entry.SourceURL)
		}
	}
}

func TestSeedPrunesStaleMetadataRows(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer store.Close()
	if err := Migrate(store); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	stale := models.Manga{
		ID:             "old-placeholder",
		Title:          "Old Placeholder",
		Author:         "Seed",
		Genres:         []string{"Action"},
		Status:         "completed",
		TotalChapters:  1,
		Description:    "Old metadata seed row",
		CoverURL:       "https://cdn.example.com/old.jpg",
		SourceProvider: "AniList metadata",
		SourceURL:      "https://anilist.co/manga/1",
		RightsStatus:   "metadata-only",
	}
	admin := stale
	admin.ID = "admin-managed"
	admin.SourceProvider = "Admin"
	seed := stale
	seed.ID = "canonical-seed"
	if err := UpsertManga(store, []models.Manga{stale, admin}); err != nil {
		t.Fatalf("insert initial rows: %v", err)
	}
	if err := upsertManga(store, []models.Manga{seed}, true); err != nil {
		t.Fatalf("upsert canonical seed: %v", err)
	}

	var count int
	if err := store.QueryRow(`SELECT COUNT(*) FROM manga WHERE id = ?`, "old-placeholder").Scan(&count); err != nil {
		t.Fatalf("count stale row: %v", err)
	}
	if count != 0 {
		t.Fatalf("stale metadata row was not pruned")
	}
	for _, id := range []string{"admin-managed", "canonical-seed"} {
		if err := store.QueryRow(`SELECT COUNT(*) FROM manga WHERE id = ?`, id).Scan(&count); err != nil {
			t.Fatalf("count %s: %v", id, err)
		}
		if count != 1 {
			t.Fatalf("%s count = %d, want 1", id, count)
		}
	}
}

func TestAdminUpsertDoesNotPruneSeedRows(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer store.Close()
	if err := Migrate(store); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	base := models.Manga{
		ID:             "canonical-seed",
		Title:          "Canonical Seed",
		Author:         "Seed",
		Genres:         []string{"Action"},
		Status:         "completed",
		TotalChapters:  1,
		Description:    "Seed row",
		CoverURL:       "https://cdn.example.com/seed.jpg",
		SourceProvider: "AniList metadata",
		SourceURL:      "https://anilist.co/manga/1",
		RightsStatus:   "metadata-only",
	}
	added := base
	added.ID = "admin-sync"
	added.Title = "Admin Sync"
	if err := upsertManga(store, []models.Manga{base}, true); err != nil {
		t.Fatalf("insert seed row: %v", err)
	}
	if err := UpsertManga(store, []models.Manga{added}); err != nil {
		t.Fatalf("admin upsert row: %v", err)
	}
	var count int
	if err := store.QueryRow(`SELECT COUNT(*) FROM manga`).Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 2 {
		t.Fatalf("row count = %d, want 2", count)
	}
}

func TestSeedCatalogMeetsPDFMetadataBaseline(t *testing.T) {
	payload, err := os.ReadFile("../../data/manga.json")
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	var entries []models.Manga
	if err := json.Unmarshal(payload, &entries); err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	if len(entries) < 200 {
		t.Fatalf("seed has %d manga records, want at least 200", len(entries))
	}

	slugPattern := regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	anilistURLPattern := regexp.MustCompile(`^https://anilist\.co/manga/[0-9]+$`)
	seenIDs := map[string]bool{}
	genreCounts := map[string]int{}
	for _, entry := range entries {
		if entry.ID == "" || !slugPattern.MatchString(entry.ID) {
			t.Fatalf("%s has unstable slug id", entry.Title)
		}
		if seenIDs[entry.ID] {
			t.Fatalf("duplicate seed id %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if entry.Title == "" {
			t.Fatalf("%s has empty title", entry.ID)
		}
		if entry.Author == "" {
			t.Fatalf("%s has empty author", entry.ID)
		}
		if len(entry.Genres) == 0 {
			t.Fatalf("%s has no genres", entry.ID)
		}
		if entry.Status == "" {
			t.Fatalf("%s has empty status", entry.ID)
		}
		if entry.TotalChapters < 0 {
			t.Fatalf("%s has negative total_chapters: %d", entry.ID, entry.TotalChapters)
		}
		if entry.Description == "" {
			t.Fatalf("%s has empty description", entry.ID)
		}
		// intent: verify seed covers stay safe without coupling legal metadata validation to one CDN path
		// status: done
		// next: keep source_provider/source_url/rights_status checks as the legal metadata gate
		// blockers: none
		// confidence: high
		coverURL, err := url.Parse(entry.CoverURL)
		if err != nil || coverURL.Scheme != "https" || coverURL.Host == "" || models.SanitizeCoverURL(entry.CoverURL) != entry.CoverURL {
			t.Fatalf("%s cover_url is not a sanitized HTTPS URL: %s", entry.ID, entry.CoverURL)
		}
		if entry.SourceProvider != "AniList metadata" {
			t.Fatalf("%s source_provider = %q, want AniList metadata", entry.ID, entry.SourceProvider)
		}
		if !anilistURLPattern.MatchString(entry.SourceURL) {
			t.Fatalf("%s source_url is not an exact AniList manga URL: %s", entry.ID, entry.SourceURL)
		}
		if entry.RightsStatus != "metadata-only" {
			t.Fatalf("%s rights_status = %q, want metadata-only", entry.ID, entry.RightsStatus)
		}
		for _, genre := range entry.Genres {
			genreCounts[genre]++
		}
	}

	for _, genre := range []string{"Shounen", "Shoujo", "Seinen", "Josei"} {
		if genreCounts[genre] < 20 {
			t.Fatalf("seed has %d %s records, want at least 20", genreCounts[genre], genre)
		}
	}
}
