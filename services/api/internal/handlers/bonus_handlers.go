package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type reviewRequest struct {
	Rating int    `json:"rating"`
	Body   string `json:"body"`
}

type friendRequest struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type friendResponseRequest struct {
	RequesterID string `json:"requester_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	Accept      bool   `json:"accept"`
	Status      string `json:"status"`
}

type recoveryRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
}

type recoveryResetRequest struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
	Password    string `json:"password"`
}

func (s *Server) listReviews(c *gin.Context) {
	mangaID := c.Param("id")
	if !s.mangaExists(mangaID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	limit := clampInt(c.DefaultQuery("limit", "20"), 1, 100)
	rows, err := s.DB.Query(`
SELECT r.id, r.manga_id, r.user_id, u.username, r.rating, r.body, r.created_at, r.updated_at
FROM reviews r
JOIN users u ON u.id = r.user_id
WHERE r.manga_id = ?
ORDER BY r.updated_at DESC
LIMIT ?`, mangaID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load reviews"})
		return
	}
	defer rows.Close()

	reviews := make([]models.Review, 0)
	for rows.Next() {
		review, err := scanReview(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid review record"})
			return
		}
		reviews = append(reviews, review)
	}
	c.JSON(http.StatusOK, gin.H{"reviews": reviews, "summary": s.reviewSummary(mangaID), "count": len(reviews)})
}

func (s *Server) submitReview(c *gin.Context) {
	userID := c.GetString("user_id")
	mangaID := c.Param("id")
	if !s.mangaExists(mangaID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "manga not found"})
		return
	}
	completed, err := s.userCompletedManga(userID, mangaID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify reading progress"})
		return
	}
	if !completed {
		c.JSON(http.StatusForbidden, gin.H{"error": "complete the manga before reviewing it"})
		return
	}

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Rating < 1 || req.Rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rating must be between 1 and 5"})
		return
	}
	if len(req.Body) < 5 || len(req.Body) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "review body must be 5-2000 characters"})
		return
	}

	reviewID := "rev_" + uuid.NewString()
	_, err = s.DB.Exec(`
INSERT INTO reviews (id, manga_id, user_id, rating, body, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(manga_id, user_id) DO UPDATE SET
	rating = excluded.rating,
	body = excluded.body,
	updated_at = CURRENT_TIMESTAMP`, reviewID, mangaID, userID, req.Rating, req.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save review"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "saved", "summary": s.reviewSummary(mangaID)})
}

func (s *Server) requestFriend(c *gin.Context) {
	userID := c.GetString("user_id")
	var req friendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	target, err := s.findUserByFriendRequest(req.UserID, req.Username, req.Email)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
		return
	}
	if target.UserID == userID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot add yourself"})
		return
	}
	if status, ok := s.friendshipStatus(userID, target.UserID); ok && status == "accepted" {
		c.JSON(http.StatusOK, gin.H{"status": "already_accepted", "friend": target})
		return
	}
	if status, ok := s.friendshipStatus(target.UserID, userID); ok {
		if status == "accepted" {
			c.JSON(http.StatusOK, gin.H{"status": "already_accepted", "friend": target})
			return
		}
		if status == "pending" {
			if _, err := s.DB.Exec(`UPDATE friendships SET status = 'accepted', updated_at = CURRENT_TIMESTAMP WHERE requester_id = ? AND addressee_id = ?`, target.UserID, userID); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to accept reciprocal request"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "accepted", "friend": target})
			return
		}
	}

	_, err = s.DB.Exec(`
INSERT INTO friendships (requester_id, addressee_id, status, created_at, updated_at)
VALUES (?, ?, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT(requester_id, addressee_id) DO UPDATE SET
	status = 'pending',
	updated_at = CURRENT_TIMESTAMP`, userID, target.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create friend request"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pending", "friend": target})
}

func (s *Server) respondFriend(c *gin.Context) {
	userID := c.GetString("user_id")
	var req friendResponseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	requester, err := s.findUserByFriendRequest(req.RequesterID, req.Username, req.Email)
	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusNotFound, gin.H{"error": "requester not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load requester"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status == "" {
		if req.Accept {
			status = "accepted"
		} else {
			status = "declined"
		}
	}
	if status != "accepted" && status != "declined" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status must be accepted or declined"})
		return
	}
	result, err := s.DB.Exec(`UPDATE friendships SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE requester_id = ? AND addressee_id = ? AND status = 'pending'`, status, requester.UserID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update friend request"})
		return
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "pending friend request not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status, "friend": requester})
}

func (s *Server) listFriends(c *gin.Context) {
	userID := c.GetString("user_id")
	rows, err := s.DB.Query(`
SELECT u.id, u.username, u.email, f.status,
       CASE WHEN f.requester_id = ? THEN 'outbound' ELSE 'inbound' END,
       f.updated_at
FROM friendships f
JOIN users u ON u.id = CASE WHEN f.requester_id = ? THEN f.addressee_id ELSE f.requester_id END
WHERE f.requester_id = ? OR f.addressee_id = ?
ORDER BY f.updated_at DESC`, userID, userID, userID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load friends"})
		return
	}
	defer rows.Close()

	all := make([]models.FriendConnection, 0)
	friends := make([]models.FriendConnection, 0)
	pendingInbound := make([]models.FriendConnection, 0)
	pendingOutbound := make([]models.FriendConnection, 0)
	for rows.Next() {
		friend, err := scanFriendConnection(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid friend record"})
			return
		}
		all = append(all, friend)
		switch {
		case friend.Status == "accepted":
			friends = append(friends, friend)
		case friend.Direction == "inbound":
			pendingInbound = append(pendingInbound, friend)
		case friend.Direction == "outbound":
			pendingOutbound = append(pendingOutbound, friend)
		}
	}
	c.JSON(http.StatusOK, gin.H{"connections": all, "friends": friends, "pending_inbound": pendingInbound, "pending_outbound": pendingOutbound})
}

func (s *Server) friendActivity(c *gin.Context) {
	userID := c.GetString("user_id")
	limit := clampInt(c.DefaultQuery("limit", "20"), 1, 100)
	friendIDs, err := s.acceptedFriendIDs(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load friends"})
		return
	}
	activity := make([]models.ActivityEvent, 0)
	for _, friendID := range friendIDs {
		progressEvents, err := s.completedProgressEvents(friendID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load progress activity"})
			return
		}
		reviewEvents, err := s.reviewActivityEvents(friendID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load review activity"})
			return
		}
		activity = append(activity, progressEvents...)
		activity = append(activity, reviewEvents...)
	}
	sort.Slice(activity, func(i, j int) bool { return activity[i].CreatedAt.After(activity[j].CreatedAt) })
	if len(activity) > limit {
		activity = activity[:limit]
	}
	c.JSON(http.StatusOK, gin.H{"activity": activity, "count": len(activity)})
}

func (s *Server) getUserStats(c *gin.Context) {
	userID := c.GetString("user_id")
	stats, err := s.buildUserStats(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stats"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func (s *Server) requestPasswordRecovery(c *gin.Context) {
	var req recoveryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	identifier := firstNonEmpty(strings.ToLower(strings.TrimSpace(req.Email)), strings.TrimSpace(req.Username))
	if identifier == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username or email is required"})
		return
	}
	var userID, email string
	if err := s.DB.QueryRow(`SELECT id, email FROM users WHERE lower(email) = ? OR username = ?`, identifier, identifier).Scan(&userID, &email); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "if_account_exists_recovery_started"})
		return
	}
	token := "rst_" + uuid.NewString()
	expiresAt := time.Now().Add(time.Hour)
	if _, err := s.DB.Exec(`INSERT INTO password_reset_tokens (id, user_id, token, expires_at) VALUES (?, ?, ?, ?)`, "prt_"+uuid.NewString(), userID, token, expiresAt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create recovery token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "recovery_started", "email": email, "token": token, "expires_at": expiresAt})
}

func (s *Server) resetPassword(c *gin.Context) {
	var req recoveryResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON body"})
		return
	}
	password := req.NewPassword
	if password == "" {
		password = req.Password
	}
	if len(password) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 8 characters"})
		return
	}
	var userID string
	var expiresAtRaw, usedAtRaw any
	if err := s.DB.QueryRow(`SELECT user_id, expires_at, used_at FROM password_reset_tokens WHERE token = ?`, strings.TrimSpace(req.Token)).Scan(&userID, &expiresAtRaw, &usedAtRaw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid recovery token"})
		return
	}
	expiresAt := parseSQLTime(expiresAtRaw)
	if expiresAt.IsZero() || time.Now().After(expiresAt) || sqlTimePresent(usedAtRaw) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "recovery token is expired or already used"})
		return
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}
	if _, err := s.DB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update password"})
		return
	}
	_, _ = s.DB.Exec(`UPDATE password_reset_tokens SET used_at = CURRENT_TIMESTAMP WHERE token = ?`, strings.TrimSpace(req.Token))
	c.JSON(http.StatusOK, gin.H{"status": "password_reset"})
}

func scanReview(row rowScanner) (models.Review, error) {
	var review models.Review
	var createdAt, updatedAt any
	if err := row.Scan(&review.ID, &review.MangaID, &review.UserID, &review.Username, &review.Rating, &review.Body, &createdAt, &updatedAt); err != nil {
		return review, err
	}
	review.CreatedAt = parseSQLTime(createdAt)
	review.UpdatedAt = parseSQLTime(updatedAt)
	return review, nil
}

func scanFriendConnection(row rowScanner) (models.FriendConnection, error) {
	var friend models.FriendConnection
	var updatedAt any
	if err := row.Scan(&friend.UserID, &friend.Username, &friend.Email, &friend.Status, &friend.Direction, &updatedAt); err != nil {
		return friend, err
	}
	friend.UpdatedAt = parseSQLTime(updatedAt)
	return friend, nil
}

func (s *Server) reviewSummary(mangaID string) models.ReviewSummary {
	var average sql.NullFloat64
	var count int
	if err := s.DB.QueryRow(`SELECT AVG(rating), COUNT(*) FROM reviews WHERE manga_id = ?`, mangaID).Scan(&average, &count); err != nil {
		return models.ReviewSummary{}
	}
	summary := models.ReviewSummary{ReviewCount: count}
	if average.Valid {
		summary.AverageRating = round2(average.Float64)
	}
	return summary
}

func (s *Server) userCompletedManga(userID, mangaID string) (bool, error) {
	var status string
	var currentChapter, totalChapters int
	if err := s.DB.QueryRow(`
SELECT p.status, p.current_chapter, m.total_chapters
FROM user_progress p
JOIN manga m ON m.id = p.manga_id
WHERE p.user_id = ? AND p.manga_id = ?`, userID, mangaID).Scan(&status, &currentChapter, &totalChapters); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return status == "completed" || (totalChapters > 0 && currentChapter >= totalChapters), nil
}

func (s *Server) findUserByFriendRequest(userID, username, email string) (models.FriendConnection, error) {
	identifierID := strings.TrimSpace(userID)
	identifierUsername := strings.TrimSpace(username)
	identifierEmail := strings.ToLower(strings.TrimSpace(email))
	if identifierID == "" && identifierUsername == "" && identifierEmail == "" {
		return models.FriendConnection{}, sql.ErrNoRows
	}
	row := s.DB.QueryRow(`
SELECT id, username, email
FROM users
WHERE (? != '' AND id = ?)
   OR (? != '' AND username = ?)
   OR (? != '' AND lower(email) = ?)
LIMIT 1`, identifierID, identifierID, identifierUsername, identifierUsername, identifierEmail, identifierEmail)
	var friend models.FriendConnection
	if err := row.Scan(&friend.UserID, &friend.Username, &friend.Email); err != nil {
		return friend, err
	}
	return friend, nil
}

func (s *Server) friendshipStatus(requesterID, addresseeID string) (string, bool) {
	var status string
	if err := s.DB.QueryRow(`SELECT status FROM friendships WHERE requester_id = ? AND addressee_id = ?`, requesterID, addresseeID).Scan(&status); err != nil {
		return "", false
	}
	return status, true
}

func (s *Server) acceptedFriendIDs(userID string) ([]string, error) {
	rows, err := s.DB.Query(`
SELECT CASE WHEN requester_id = ? THEN addressee_id ELSE requester_id END
FROM friendships
WHERE status = 'accepted' AND (requester_id = ? OR addressee_id = ?)`, userID, userID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Server) completedProgressEvents(userID string, limit int) ([]models.ActivityEvent, error) {
	rows, err := s.DB.Query(`
SELECT u.id, u.username, m.id, m.title, p.current_chapter, p.status, p.updated_at
FROM user_progress p
JOIN users u ON u.id = p.user_id
JOIN manga m ON m.id = p.manga_id
WHERE p.user_id = ? AND p.status = 'completed'
ORDER BY p.updated_at DESC
LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]models.ActivityEvent, 0)
	for rows.Next() {
		var event models.ActivityEvent
		var createdAt any
		event.Type = "completed"
		if err := rows.Scan(&event.UserID, &event.Username, &event.MangaID, &event.Title, &event.Chapter, &event.Status, &createdAt); err != nil {
			return nil, err
		}
		event.CreatedAt = parseSQLTime(createdAt)
		events = append(events, event)
	}
	return events, nil
}

func (s *Server) reviewActivityEvents(userID string, limit int) ([]models.ActivityEvent, error) {
	rows, err := s.DB.Query(`
SELECT u.id, u.username, m.id, m.title, r.rating, r.body, r.updated_at
FROM reviews r
JOIN users u ON u.id = r.user_id
JOIN manga m ON m.id = r.manga_id
WHERE r.user_id = ?
ORDER BY r.updated_at DESC
LIMIT ?`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := make([]models.ActivityEvent, 0)
	for rows.Next() {
		var event models.ActivityEvent
		var createdAt any
		event.Type = "review"
		if err := rows.Scan(&event.UserID, &event.Username, &event.MangaID, &event.Title, &event.Rating, &event.Body, &createdAt); err != nil {
			return nil, err
		}
		event.CreatedAt = parseSQLTime(createdAt)
		events = append(events, event)
	}
	return events, nil
}

func (s *Server) buildUserStats(userID string) (models.UserStats, error) {
	rows, err := s.DB.Query(`
SELECT p.status, p.current_chapter, m.genres, p.updated_at
FROM user_progress p
JOIN manga m ON m.id = p.manga_id
WHERE p.user_id = ?`, userID)
	if err != nil {
		return models.UserStats{}, err
	}
	defer rows.Close()

	stats := models.UserStats{}
	genreCounts := map[string]int{}
	statusCounts := map[string]int{}
	trendCounts := map[string]int{}
	for rows.Next() {
		var status, genresJSON string
		var chapter int
		var updatedAt any
		if err := rows.Scan(&status, &chapter, &genresJSON, &updatedAt); err != nil {
			return stats, err
		}
		stats.LibraryCount++
		stats.TotalChaptersRead += chapter
		if status == "completed" {
			stats.CompletedCount++
		}
		if status == "reading" {
			stats.ReadingCount++
		}
		statusCounts[status]++
		var genres []string
		_ = jsonUnmarshalStringList(genresJSON, &genres)
		for _, genre := range genres {
			genreCounts[genre]++
		}
		if updated := parseSQLTime(updatedAt); !updated.IsZero() {
			trendCounts[updated.Format("2006-01-02")] += chapter
		}
	}
	stats.FavoriteGenres = topGenreStats(genreCounts, 5)
	stats.StatusBreakdown = statusStats(statusCounts)
	stats.Trends = trendPoints(trendCounts)
	stats.AverageRating, stats.ReviewCount = s.userReviewAverage(userID)
	return stats, nil
}

func jsonUnmarshalStringList(raw string, target *[]string) error {
	return json.Unmarshal([]byte(raw), target)
}

func (s *Server) userReviewAverage(userID string) (float64, int) {
	var average sql.NullFloat64
	var count int
	if err := s.DB.QueryRow(`SELECT AVG(rating), COUNT(*) FROM reviews WHERE user_id = ?`, userID).Scan(&average, &count); err != nil {
		return 0, 0
	}
	if !average.Valid {
		return 0, count
	}
	return round2(average.Float64), count
}

func topGenreStats(counts map[string]int, limit int) []models.GenreStat {
	values := make([]models.GenreStat, 0, len(counts))
	for genre, count := range counts {
		values = append(values, models.GenreStat{Genre: genre, Count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count == values[j].Count {
			return values[i].Genre < values[j].Genre
		}
		return values[i].Count > values[j].Count
	})
	if len(values) > limit {
		return values[:limit]
	}
	return values
}

func statusStats(counts map[string]int) []models.StatusStat {
	values := make([]models.StatusStat, 0, len(counts))
	for status, count := range counts {
		values = append(values, models.StatusStat{Status: status, Count: count})
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Count == values[j].Count {
			return values[i].Status < values[j].Status
		}
		return values[i].Count > values[j].Count
	})
	return values
}

func trendPoints(counts map[string]int) []models.TrendPoint {
	values := make([]models.TrendPoint, 0, len(counts))
	for date, chapters := range counts {
		values = append(values, models.TrendPoint{Date: date, Chapters: chapters})
	}
	sort.Slice(values, func(i, j int) bool { return values[i].Date < values[j].Date })
	return values
}

func sqlTimePresent(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case time.Time:
		return !typed.IsZero()
	case string:
		return strings.TrimSpace(typed) != ""
	case []byte:
		return strings.TrimSpace(string(typed)) != ""
	default:
		return true
	}
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}
