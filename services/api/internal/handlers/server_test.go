package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"

	"github.com/gin-gonic/gin"
)

func TestHTTPApplicationFlow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := database.SeedManga(db, filepath.Join("..", "..", "data", "manga.json")); err != nil {
		t.Fatalf("seed manga: %v", err)
	}

	server := NewServer(db, auth.NewJWTManager("test-secret"))

	registerBody := `{"username":"demo_user","email":"demo@example.com","password":"Password123"}`
	register := perform(server.Router, http.MethodPost, "/auth/register", registerBody, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register status = %d, body = %s", register.Code, register.Body.String())
	}

	login := perform(server.Router, http.MethodPost, "/auth/login", `{"username":"demo_user","password":"Password123"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", login.Code, login.Body.String())
	}
	var loginPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if loginPayload.Token == "" {
		t.Fatal("expected token")
	}

	search := perform(server.Router, http.MethodGet, "/manga?q=naruto", "", "")
	if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Naruto") || !strings.Contains(search.Body.String(), "source_provider") {
		t.Fatalf("search failed: %d %s", search.Code, search.Body.String())
	}

	sources := perform(server.Router, http.MethodGet, "/sources", "", "")
	if sources.Code != http.StatusOK || !strings.Contains(sources.Body.String(), "AniList GraphQL") || !strings.Contains(sources.Body.String(), "MangaDex API") {
		t.Fatalf("sources failed: %d %s", sources.Code, sources.Body.String())
	}
	if !strings.Contains(sources.Body.String(), "/sources/anilist/:id") || !strings.Contains(sources.Body.String(), "/sources/mangadex/:id") || strings.Contains(sources.Body.String(), "/sources/anilist/search?q=") {
		t.Fatalf("sources should advertise exact source metadata endpoints, got %s", sources.Body.String())
	}

	chapters := perform(server.Router, http.MethodGet, "/manga/naruto/chapters", "", "")
	if chapters.Code != http.StatusOK || !strings.Contains(chapters.Body.String(), `"count":0`) || strings.Contains(chapters.Body.String(), "source-preview") {
		t.Fatalf("empty chapter listing failed: %d %s", chapters.Code, chapters.Body.String())
	}
	readerChapter := perform(server.Router, http.MethodGet, "/manga/naruto/chapters/1", "", "")
	if readerChapter.Code != http.StatusNotFound {
		t.Fatalf("missing reader chapter should 404: %d %s", readerChapter.Code, readerChapter.Body.String())
	}

	addBody := `{"manga_id":"naruto","status":"reading","current_chapter":1}`
	add := perform(server.Router, http.MethodPost, "/users/library", addBody, loginPayload.Token)
	if add.Code != http.StatusOK {
		t.Fatalf("add library status = %d, body = %s", add.Code, add.Body.String())
	}

	progressBody := `{"manga_id":"naruto","chapter":2,"status":"reading"}`
	progress := perform(server.Router, http.MethodPut, "/users/progress", progressBody, loginPayload.Token)
	if progress.Code != http.StatusOK || !strings.Contains(progress.Body.String(), "tcp_broadcast") {
		t.Fatalf("progress failed: %d %s", progress.Code, progress.Body.String())
	}

	library := perform(server.Router, http.MethodGet, "/users/library", "", loginPayload.Token)
	if library.Code != http.StatusOK || !strings.Contains(library.Body.String(), "Naruto") {
		t.Fatalf("library failed: %d %s", library.Code, library.Body.String())
	}

	remove := perform(server.Router, http.MethodDelete, "/users/library/naruto", "", loginPayload.Token)
	if remove.Code != http.StatusOK {
		t.Fatalf("remove library failed: %d %s", remove.Code, remove.Body.String())
	}
}

func TestAdminMangaAndChapterCRUD(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServerWithConfig(db, auth.NewJWTManager("test-secret"), Config{
		AllowedOrigin:    "*",
		AdminSyncToken:   "admin-token",
		ChapterUploadDir: filepath.Join(t.TempDir(), "chapter-pages"),
	})

	blocked := perform(server.Router, http.MethodPost, "/admin/manga", `{"title":"Blocked"}`, "")
	if blocked.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized admin request, got %d %s", blocked.Code, blocked.Body.String())
	}

	createManga := performAdmin(server.Router, http.MethodPost, "/admin/manga", `{
		"id":"admin-series",
		"title":"Admin Series",
		"author":"Admin Author",
		"genres":["Action","Action","Drama"],
		"status":"ongoing",
		"description":"Managed in admin",
		"cover_url":"https://cdn.example.com/media/covers/stale.svg",
		"source_provider":"Admin",
		"rights_status":"licensed"
	}`, "admin-token")
	if createManga.Code != http.StatusCreated {
		t.Fatalf("create manga failed: %d %s", createManga.Code, createManga.Body.String())
	}
	if strings.Contains(createManga.Body.String(), "stale.svg") {
		t.Fatalf("expected stale svg cover url to be sanitized: %s", createManga.Body.String())
	}

	updateManga := performAdmin(server.Router, http.MethodPut, "/admin/manga/admin-series", `{"status":"completed","total_chapters":2}`, "admin-token")
	if updateManga.Code != http.StatusOK || !strings.Contains(updateManga.Body.String(), `"status":"completed"`) {
		t.Fatalf("update manga failed: %d %s", updateManga.Code, updateManga.Body.String())
	}

	createChapter := performAdmin(server.Router, http.MethodPost, "/admin/manga/admin-series/chapters", `{
		"id":"chapter-1",
		"title":"Opening",
		"chapter_number":1,
		"page_urls":["https://cdn.example.com/pages/1.jpg","javascript:alert(1)"],
		"publish_status":"published"
	}`, "admin-token")
	if createChapter.Code != http.StatusCreated {
		t.Fatalf("create chapter failed: %d %s", createChapter.Code, createChapter.Body.String())
	}
	if strings.Contains(createChapter.Body.String(), "javascript:") || !strings.Contains(createChapter.Body.String(), `"page_count":1`) {
		t.Fatalf("chapter pages were not sanitized/count updated: %s", createChapter.Body.String())
	}

	listChapters := perform(server.Router, http.MethodGet, "/manga/admin-series/chapters", "", "")
	if listChapters.Code != http.StatusOK || !strings.Contains(listChapters.Body.String(), "Opening") {
		t.Fatalf("list chapters failed: %d %s", listChapters.Code, listChapters.Body.String())
	}

	getChapter := perform(server.Router, http.MethodGet, "/manga/admin-series/chapters/chapter-1", "", "")
	if getChapter.Code != http.StatusOK || !strings.Contains(getChapter.Body.String(), `"chapter_number":1`) {
		t.Fatalf("get chapter failed: %d %s", getChapter.Code, getChapter.Body.String())
	}

	updateChapter := performAdmin(server.Router, http.MethodPut, "/admin/manga/admin-series/chapters/chapter-1", `{"title":"Opening Revised","page_urls":[]}`, "admin-token")
	if updateChapter.Code != http.StatusOK || !strings.Contains(updateChapter.Body.String(), "Opening Revised") || !strings.Contains(updateChapter.Body.String(), `"page_count":0`) {
		t.Fatalf("update chapter failed: %d %s", updateChapter.Code, updateChapter.Body.String())
	}

	deleteChapter := performAdmin(server.Router, http.MethodDelete, "/admin/manga/admin-series/chapters/chapter-1", "", "admin-token")
	if deleteChapter.Code != http.StatusOK {
		t.Fatalf("delete chapter failed: %d %s", deleteChapter.Code, deleteChapter.Body.String())
	}

	deleteManga := performAdmin(server.Router, http.MethodDelete, "/admin/manga/admin-series", "", "admin-token")
	if deleteManga.Code != http.StatusOK {
		t.Fatalf("delete manga failed: %d %s", deleteManga.Code, deleteManga.Body.String())
	}
}

func TestBonusUseCasesReviewFriendsStatsAndRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := database.UpsertManga(db, []models.Manga{
		{
			ID:              "bonus-series",
			Title:           "Bonus Series",
			Author:          "Demo Author",
			Genres:          []string{"Drama", "Josei"},
			Status:          "completed",
			TotalChapters:   10,
			Description:     "A series used for review, stats, and friend activity demos.",
			CoverURL:        "https://cdn.example.com/bonus.jpg",
			SourceProvider:  "MangaHub test",
			RightsStatus:    "metadata-only",
			PublicationYear: 2020,
		},
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}
	server := NewServer(db, auth.NewJWTManager("test-secret"))

	aliceID, aliceToken := registerAndLogin(t, server.Router, "alice_reader", "alice@example.com")
	bobID, bobToken := registerAndLogin(t, server.Router, "bob_reader", "bob@example.com")

	add := perform(server.Router, http.MethodPost, "/users/library", `{"manga_id":"bonus-series","status":"completed","current_chapter":10}`, aliceToken)
	if add.Code != http.StatusOK {
		t.Fatalf("add completed library entry failed: %d %s", add.Code, add.Body.String())
	}
	review := perform(server.Router, http.MethodPost, "/manga/bonus-series/reviews", `{"rating":5,"body":"Excellent finale and strong pacing."}`, aliceToken)
	if review.Code != http.StatusOK || !strings.Contains(review.Body.String(), `"review_count":1`) {
		t.Fatalf("submit review failed: %d %s", review.Code, review.Body.String())
	}
	listReviews := perform(server.Router, http.MethodGet, "/manga/bonus-series/reviews", "", "")
	if listReviews.Code != http.StatusOK || !strings.Contains(listReviews.Body.String(), "alice_reader") {
		t.Fatalf("list reviews failed: %d %s", listReviews.Code, listReviews.Body.String())
	}
	advancedSearch := perform(server.Router, http.MethodGet, "/manga?min_rating=4&year=2020&sort=rating", "", "")
	if advancedSearch.Code != http.StatusOK || !strings.Contains(advancedSearch.Body.String(), "Bonus Series") || !strings.Contains(advancedSearch.Body.String(), `"average_rating":5`) {
		t.Fatalf("advanced search failed: %d %s", advancedSearch.Code, advancedSearch.Body.String())
	}

	friendRequest := perform(server.Router, http.MethodPost, "/users/friends/request", `{"username":"alice_reader"}`, bobToken)
	if friendRequest.Code != http.StatusOK || !strings.Contains(friendRequest.Body.String(), "pending") {
		t.Fatalf("friend request failed: %d %s", friendRequest.Code, friendRequest.Body.String())
	}
	friendAccept := perform(server.Router, http.MethodPost, "/users/friends/respond", `{"requester_id":"`+bobID+`","status":"accepted"}`, aliceToken)
	if friendAccept.Code != http.StatusOK || !strings.Contains(friendAccept.Body.String(), "accepted") {
		t.Fatalf("friend accept failed: %d %s", friendAccept.Code, friendAccept.Body.String())
	}
	activity := perform(server.Router, http.MethodGet, "/users/activity", "", bobToken)
	if activity.Code != http.StatusOK || !strings.Contains(activity.Body.String(), "Excellent finale") || !strings.Contains(activity.Body.String(), aliceID) {
		t.Fatalf("friend activity failed: %d %s", activity.Code, activity.Body.String())
	}
	stats := perform(server.Router, http.MethodGet, "/users/stats", "", aliceToken)
	if stats.Code != http.StatusOK || !strings.Contains(stats.Body.String(), `"total_chapters_read":10`) || !strings.Contains(stats.Body.String(), `"review_count":1`) {
		t.Fatalf("stats failed: %d %s", stats.Code, stats.Body.String())
	}

	recovery := perform(server.Router, http.MethodPost, "/auth/recovery/request", `{"email":"alice@example.com"}`, "")
	if recovery.Code != http.StatusOK {
		t.Fatalf("recovery request failed: %d %s", recovery.Code, recovery.Body.String())
	}
	var recoveryPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(recovery.Body.Bytes(), &recoveryPayload); err != nil || recoveryPayload.Token == "" {
		t.Fatalf("decode recovery token: %v %s", err, recovery.Body.String())
	}
	reset := perform(server.Router, http.MethodPost, "/auth/recovery/reset", `{"token":"`+recoveryPayload.Token+`","new_password":"Password456"}`, "")
	if reset.Code != http.StatusOK {
		t.Fatalf("password reset failed: %d %s", reset.Code, reset.Body.String())
	}
	loginNewPassword := perform(server.Router, http.MethodPost, "/auth/login", `{"username":"alice_reader","password":"Password456"}`, "")
	if loginNewPassword.Code != http.StatusOK {
		t.Fatalf("login after reset failed: %d %s", loginNewPassword.Code, loginNewPassword.Body.String())
	}
}

func TestAdminRoleCanMutateWithoutSharedToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	server := NewServerWithConfig(db, auth.NewJWTManager("test-secret"), Config{
		AllowedOrigin: "*",
		AdminEmails:   []string{"admin@example.com"},
	})

	registerAdmin := perform(server.Router, http.MethodPost, "/auth/register", `{"username":"admin_user","email":"admin@example.com","password":"Password123"}`, "")
	if registerAdmin.Code != http.StatusCreated || !strings.Contains(registerAdmin.Body.String(), `"role":"admin"`) {
		t.Fatalf("admin registration failed: %d %s", registerAdmin.Code, registerAdmin.Body.String())
	}
	loginAdmin := perform(server.Router, http.MethodPost, "/auth/login", `{"username":"admin_user","password":"Password123"}`, "")
	if loginAdmin.Code != http.StatusOK || !strings.Contains(loginAdmin.Body.String(), `"role":"admin"`) {
		t.Fatalf("admin login failed: %d %s", loginAdmin.Code, loginAdmin.Body.String())
	}
	var loginPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginAdmin.Body.Bytes(), &loginPayload); err != nil {
		t.Fatalf("decode admin login: %v", err)
	}

	createManga := perform(server.Router, http.MethodPost, "/admin/manga", `{
		"id":"role-admin-series",
		"title":"Role Admin Series",
		"author":"Admin Author",
		"genres":["Action"],
		"status":"ongoing",
		"description":"Managed by an admin user",
		"cover_url":"https://cdn.example.com/cover.jpg",
		"source_provider":"Admin",
		"rights_status":"licensed"
	}`, loginPayload.Token)
	if createManga.Code != http.StatusCreated {
		t.Fatalf("admin bearer create manga failed: %d %s", createManga.Code, createManga.Body.String())
	}

	registerReader := perform(server.Router, http.MethodPost, "/auth/register", `{"username":"reader_user","email":"reader@example.com","password":"Password123"}`, "")
	if registerReader.Code != http.StatusCreated || !strings.Contains(registerReader.Body.String(), `"role":"reader"`) {
		t.Fatalf("reader registration failed: %d %s", registerReader.Code, registerReader.Body.String())
	}
	loginReader := perform(server.Router, http.MethodPost, "/auth/login", `{"username":"reader_user","password":"Password123"}`, "")
	if loginReader.Code != http.StatusOK {
		t.Fatalf("reader login failed: %d %s", loginReader.Code, loginReader.Body.String())
	}
	var readerPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginReader.Body.Bytes(), &readerPayload); err != nil {
		t.Fatalf("decode reader login: %v", err)
	}
	readerCreate := perform(server.Router, http.MethodPost, "/admin/manga", `{"id":"blocked","title":"Blocked"}`, readerPayload.Token)
	if readerCreate.Code != http.StatusUnauthorized && readerCreate.Code != http.StatusForbidden {
		t.Fatalf("expected reader admin request to fail, got %d %s", readerCreate.Code, readerCreate.Body.String())
	}
}

func TestHealthReflectsConfiguredServiceStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	statuses := map[string]string{
		"tcp":       "online",
		"udp":       "configured",
		"grpc":      "error",
		"websocket": "online",
	}
	server := NewServerWithConfig(db, auth.NewJWTManager("test-secret"), Config{
		AllowedOrigin: "*",
		ServiceStatusFunc: func() map[string]string {
			return statuses
		},
	})

	health := perform(server.Router, http.MethodGet, "/health", "", "")
	if health.Code != http.StatusOK {
		t.Fatalf("health failed: %d %s", health.Code, health.Body.String())
	}
	body := health.Body.String()
	for _, expected := range []string{`"tcp":"online"`, `"udp":"configured"`, `"grpc":"error"`, `"websocket":"online"`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("health did not include %s: %s", expected, body)
		}
	}
	retiredStatuses := []string{"in" + "active", "off" + "line"}
	for _, status := range retiredStatuses {
		if strings.Contains(body, status) {
			t.Fatalf("health should not report implemented services with retired status %q: %s", status, body)
		}
	}
}

func TestAdminRoutesRequireConfiguredToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	server := NewServer(db, auth.NewJWTManager("test-secret"))

	resp := perform(server.Router, http.MethodPost, "/admin/manga", `{"title":"Blocked"}`, "")
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected admin route without credentials to return 401, got %d %s", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "admin role") || !strings.Contains(resp.Body.String(), "ADMIN_SYNC_TOKEN") {
		t.Fatalf("admin route should explain required role/token: %s", resp.Body.String())
	}
}

func TestCORSAllowsLocalhostAndLoopbackOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	server := NewServerWithConfig(db, auth.NewJWTManager("test-secret"), Config{AllowedOrigin: "http://localhost:3000"})

	req := httptest.NewRequest(http.MethodOptions, "/users/me", nil)
	req.Header.Set("Origin", "http://127.0.0.1:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	recorder := httptest.NewRecorder()
	server.Router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:3000" {
		t.Fatalf("allow origin = %q, want loopback origin", got)
	}
	if got := recorder.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("vary = %q, want Origin", got)
	}
}

func TestCatalogSearchAcceptsAniListStatusAliasesAndPreservesCoverURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	smallCover := "https://s4.anilist.co/file/anilistcdn/media/manga/cover/small/b79865-rwKmISrhJJyP.jpg"
	if err := database.UpsertManga(db, []models.Manga{
		{
			ID:             "ongoing-series",
			Title:          "Ongoing Series",
			Author:         "AniList",
			Genres:         []string{"Action"},
			Status:         "ongoing",
			Description:    "Ongoing metadata",
			CoverURL:       smallCover,
			SourceProvider: "AniList GraphQL",
			SourceURL:      "https://anilist.co/manga/79865",
			RightsStatus:   "metadata-only",
		},
		{
			ID:             "completed-series",
			Title:          "Completed Series",
			Author:         "AniList",
			Genres:         []string{"Action"},
			Status:         "completed",
			Description:    "Completed metadata",
			CoverURL:       "https://cdn.example.com/completed.jpg",
			SourceProvider: "AniList GraphQL",
			SourceURL:      "https://anilist.co/manga/1",
			RightsStatus:   "metadata-only",
		},
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	server := NewServer(db, auth.NewJWTManager("test-secret"))
	search := perform(server.Router, http.MethodGet, "/manga?status=RELEASING", "", "")
	body := search.Body.String()
	if search.Code != http.StatusOK || !strings.Contains(body, "Ongoing Series") || strings.Contains(body, "Completed Series") {
		t.Fatalf("status-filtered search failed: %d %s", search.Code, body)
	}
	if !strings.Contains(body, smallCover) || strings.Contains(body, "/cover/large/b79865-rwKmISrhJJyP.jpg") {
		t.Fatalf("cover URL should be preserved unless AniList GraphQL returns a large URL: %s", body)
	}
}

func TestCatalogSearchHandlesConcurrentHTTPRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	if err := database.UpsertManga(db, []models.Manga{
		{
			ID:             "concurrent-naruto",
			Title:          "Concurrent Naruto",
			Author:         "Masashi Kishimoto",
			Genres:         []string{"Action", "Shounen"},
			Status:         "completed",
			TotalChapters:  700,
			Description:    "A catalog row used to verify concurrent HTTP search behavior.",
			CoverURL:       "https://s4.anilist.co/file/anilistcdn/media/manga/cover/large/bx20-YJvLbgJQPCoI.jpg",
			SourceProvider: "AniList metadata",
			SourceURL:      "https://anilist.co/manga/20",
			RightsStatus:   "metadata-only",
		},
	}); err != nil {
		t.Fatalf("upsert manga: %v", err)
	}

	server := NewServer(db, auth.NewJWTManager("test-secret"))
	const requests = 24
	var wg sync.WaitGroup
	errs := make(chan string, requests)
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			search := perform(server.Router, http.MethodGet, "/manga?q=naruto&limit=5", "", "")
			if search.Code != http.StatusOK || !strings.Contains(search.Body.String(), "Concurrent Naruto") || !strings.Contains(search.Body.String(), "metadata-only") {
				errs <- search.Body.String()
			}
		}()
	}
	wg.Wait()
	close(errs)
	if body, ok := <-errs; ok {
		t.Fatalf("concurrent search returned unexpected response: %s", body)
	}
}

func TestAniListStatusBucketNormalization(t *testing.T) {
	statuses, err := normalizeAniListStatusBuckets([]string{"ongoing", "HIATUS", "cancelled", "RELEASING"})
	if err != nil {
		t.Fatalf("normalize status buckets: %v", err)
	}
	want := []string{"RELEASING", "HIATUS", "CANCELLED"}
	if len(statuses) != len(want) {
		t.Fatalf("statuses = %#v, want %#v", statuses, want)
	}
	for i := range want {
		if statuses[i] != want[i] {
			t.Fatalf("statuses = %#v, want %#v", statuses, want)
		}
	}
	if _, err := normalizeAniListStatusBuckets([]string{"bad-status"}); err == nil {
		t.Fatal("expected invalid status to fail")
	}
}

func TestMangaDexMappingAndStatusNormalization(t *testing.T) {
	statuses, err := normalizeMangaDexStatusBuckets([]string{"ongoing", "hiatus", "cancelled", "ongoing"})
	if err != nil {
		t.Fatalf("normalize MangaDex status buckets: %v", err)
	}
	wantStatuses := []string{"ongoing", "hiatus", "cancelled"}
	if len(statuses) != len(wantStatuses) {
		t.Fatalf("statuses = %#v, want %#v", statuses, wantStatuses)
	}
	for i := range wantStatuses {
		if statuses[i] != wantStatuses[i] {
			t.Fatalf("statuses = %#v, want %#v", statuses, wantStatuses)
		}
	}
	if _, err := normalizeMangaDexStatusBuckets([]string{"bad-status"}); err == nil {
		t.Fatal("expected invalid MangaDex status to fail")
	}

	entity := mangaDexEntity{
		ID: "304ceac3-8cdb-4fe7-acf7-2b6ff7a60613",
		Attributes: mangaDexAttributes{
			Title:       map[string]string{"en": "Attack on Titan"},
			Description: map[string]string{"en": "<p>Humanity fights giants.</p>"},
			Status:      "completed",
			LastChapter: "139",
		},
		Relationships: []mangaDexRelationship{
			{Type: "author", Attributes: struct {
				Name     string `json:"name"`
				FileName string `json:"fileName"`
			}{Name: "Isayama Hajime"}},
			{Type: "cover_art", Attributes: struct {
				Name     string `json:"name"`
				FileName string `json:"fileName"`
			}{FileName: "29f82b1d-b37f-455a-b630-e42bccb1422a.jpg"}},
		},
	}
	manga := mapMangaDexEntity(entity)
	if manga.ID != "mangadex-304ceac3-8cdb-4fe7-acf7-2b6ff7a60613" || manga.SourceProvider != "MangaDex API" || manga.RightsStatus != "metadata-only" {
		t.Fatalf("unexpected MangaDex mapping: %#v", manga)
	}
	if manga.TotalChapters != 139 || manga.Author != "Isayama Hajime" || !strings.Contains(manga.CoverURL, "/covers/304ceac3-8cdb-4fe7-acf7-2b6ff7a60613/") || !strings.HasSuffix(manga.CoverURL, ".jpg.512.jpg") {
		t.Fatalf("unexpected MangaDex metadata: %#v", manga)
	}
	if !isMangaDexCoverURLForID(manga.CoverURL, "304ceac3-8cdb-4fe7-acf7-2b6ff7a60613") {
		t.Fatalf("expected MangaDex cover URL to validate: %s", manga.CoverURL)
	}
	if isMangaDexCoverURLForID("https://uploads.mangadex.org/data/hash/page.png", "304ceac3-8cdb-4fe7-acf7-2b6ff7a60613") {
		t.Fatal("expected MangaDex chapter image URL to be rejected as a cover")
	}
	if extractMangaDexMangaID(manga.SourceURL) != "304ceac3-8cdb-4fe7-acf7-2b6ff7a60613" {
		t.Fatalf("failed to extract MangaDex source id from %s", manga.SourceURL)
	}
}

func TestAniListStatusValidationRejectsBadStatusWithoutNetwork(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := database.Open(filepath.Join(t.TempDir(), "mangahub.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	server := NewServerWithConfig(db, auth.NewJWTManager("test-secret"), Config{
		AllowedOrigin:  "*",
		AdminSyncToken: "admin-token",
	})

	search := perform(server.Router, http.MethodGet, "/sources/anilist/search?status=bad-status", "", "")
	if search.Code != http.StatusBadRequest {
		t.Fatalf("expected bad source status to return 400, got %d %s", search.Code, search.Body.String())
	}
	sync := performAdmin(server.Router, http.MethodPost, "/admin/sync/anilist", `{"statuses":["ongoing","bad-status"]}`, "admin-token")
	if sync.Code != http.StatusBadRequest || !strings.Contains(sync.Body.String(), "bad-status") {
		t.Fatalf("expected bad sync status to return 400, got %d %s", sync.Code, sync.Body.String())
	}
}

func registerAndLogin(t *testing.T, router http.Handler, username, email string) (string, string) {
	t.Helper()
	register := perform(router, http.MethodPost, "/auth/register", `{"username":"`+username+`","email":"`+email+`","password":"Password123"}`, "")
	if register.Code != http.StatusCreated {
		t.Fatalf("register %s failed: %d %s", username, register.Code, register.Body.String())
	}
	var registerPayload struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(register.Body.Bytes(), &registerPayload); err != nil {
		t.Fatalf("decode register: %v", err)
	}
	login := perform(router, http.MethodPost, "/auth/login", `{"username":"`+username+`","password":"Password123"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d %s", username, login.Code, login.Body.String())
	}
	var loginPayload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &loginPayload); err != nil || loginPayload.Token == "" {
		t.Fatalf("decode login: %v %s", err, login.Body.String())
	}
	return registerPayload.User.ID, loginPayload.Token
}

func perform(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func performAdmin(router http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("X-Admin-Sync-Token", token)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}
