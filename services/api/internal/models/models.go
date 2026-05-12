package models

import (
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Manga struct {
	ID              string                `json:"id"`
	Title           string                `json:"title"`
	Author          string                `json:"author"`
	Genres          []string              `json:"genres"`
	Status          string                `json:"status"`
	TotalChapters   int                   `json:"total_chapters"`
	Description     string                `json:"description"`
	CoverURL        string                `json:"cover_url"`
	SourceProvider  string                `json:"source_provider"`
	SourceURL       string                `json:"source_url"`
	RightsStatus    string                `json:"rights_status"`
	CoverLargeURL   string                `json:"cover_large_url,omitempty"`
	BannerURL       string                `json:"banner_url,omitempty"`
	StartDate       string                `json:"start_date,omitempty"`
	EndDate         string                `json:"end_date,omitempty"`
	Format          string                `json:"format,omitempty"`
	Volumes         int                   `json:"volumes,omitempty"`
	MeanScore       int                   `json:"mean_score,omitempty"`
	AverageScore    int                   `json:"average_score,omitempty"`
	Popularity      int                   `json:"popularity,omitempty"`
	Favourites      int                   `json:"favourites,omitempty"`
	CountryOfOrigin string                `json:"country_of_origin,omitempty"`
	IsLicensed      bool                  `json:"is_licensed,omitempty"`
	Synonyms        []string              `json:"synonyms,omitempty"`
	Tags            []MangaTag            `json:"tags,omitempty"`
	Relations       []MangaRelation       `json:"relations,omitempty"`
	Recommendations []MangaRecommendation `json:"recommendations,omitempty"`
	Characters      []MangaCredit         `json:"characters,omitempty"`
	Staff           []MangaCredit         `json:"staff,omitempty"`
	ExternalLinks   []MangaExternalLink   `json:"external_links,omitempty"`
	Rankings        []MangaRanking        `json:"rankings,omitempty"`
}

type MangaTag struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	Rank             int    `json:"rank"`
	IsMediaSpoiler   bool   `json:"is_media_spoiler"`
	IsGeneralSpoiler bool   `json:"is_general_spoiler"`
}

type MangaRelation struct {
	ID           int    `json:"id"`
	RelationType string `json:"relation_type"`
	Title        string `json:"title"`
	Format       string `json:"format,omitempty"`
	Type         string `json:"type,omitempty"`
	Status       string `json:"status,omitempty"`
	CoverURL     string `json:"cover_url,omitempty"`
	BannerURL    string `json:"banner_url,omitempty"`
}

type MangaRecommendation struct {
	ID         int    `json:"id"`
	Rating     int    `json:"rating"`
	UserRating string `json:"user_rating,omitempty"`
	Title      string `json:"title"`
	Format     string `json:"format,omitempty"`
	Type       string `json:"type,omitempty"`
	Status     string `json:"status,omitempty"`
	CoverURL   string `json:"cover_url,omitempty"`
	BannerURL  string `json:"banner_url,omitempty"`
}

type MangaCredit struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Role     string `json:"role,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Language string `json:"language,omitempty"`
}

type MangaExternalLink struct {
	ID       int    `json:"id"`
	Site     string `json:"site"`
	URL      string `json:"url"`
	Type     string `json:"type,omitempty"`
	Language string `json:"language,omitempty"`
	Color    string `json:"color,omitempty"`
	Icon     string `json:"icon,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type MangaRanking struct {
	ID      int    `json:"id"`
	Rank    int    `json:"rank"`
	Type    string `json:"type"`
	Format  string `json:"format,omitempty"`
	Year    int    `json:"year,omitempty"`
	Season  string `json:"season,omitempty"`
	AllTime bool   `json:"all_time"`
	Context string `json:"context"`
}

type Chapter struct {
	ID            string    `json:"id"`
	MangaID       string    `json:"manga_id"`
	Title         string    `json:"title"`
	ChapterNumber float64   `json:"chapter_number"`
	PageURLs      []string  `json:"page_urls"`
	PageCount     int       `json:"page_count"`
	PublishStatus string    `json:"publish_status"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type User struct {
	ID        string    `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

type LibraryEntry struct {
	MangaID        string    `json:"manga_id"`
	Title          string    `json:"title"`
	Author         string    `json:"author"`
	Genres         []string  `json:"genres"`
	ReadingStatus  string    `json:"reading_status"`
	CurrentChapter int       `json:"current_chapter"`
	TotalChapters  int       `json:"total_chapters"`
	CoverURL       string    `json:"cover_url"`
	SourceProvider string    `json:"source_provider"`
	SourceURL      string    `json:"source_url"`
	RightsStatus   string    `json:"rights_status"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ProgressUpdate struct {
	UserID    string `json:"user_id"`
	MangaID   string `json:"manga_id"`
	Chapter   int    `json:"chapter"`
	Timestamp int64  `json:"timestamp"`
}

type Notification struct {
	Type      string `json:"type"`
	MangaID   string `json:"manga_id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type ChatMessage struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

func SanitizeCoverURL(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	if regexp.MustCompile(`(?i)^/media/covers/[^/?#]+\.svg$`).MatchString(parsed.Path) {
		return ""
	}
	return trimmed
}
