package protocols

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"
	mangahubpb "mangahub/services/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ProgressBroadcaster interface {
	BroadcastProgress(models.ProgressUpdate) int
}

type GRPCServer struct {
	mangahubpb.UnimplementedMangaServiceServer

	Addr string
	DB   *database.Store
	JWT  *auth.JWTManager
	Sync ProgressBroadcaster
}

func NewGRPCServer(addr string, db *database.Store, jwt *auth.JWTManager, sync ProgressBroadcaster) *GRPCServer {
	return &GRPCServer{Addr: addr, DB: db, JWT: jwt, Sync: sync}
}

func (s *GRPCServer) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	server := grpc.NewServer()
	mangahubpb.RegisterMangaServiceServer(server, s)
	go func() {
		<-ctx.Done()
		server.GracefulStop()
		_ = lis.Close()
	}()
	log.Printf("MangaHub gRPC manga service listening on %s", lis.Addr().String())
	if err := server.Serve(lis); err != nil && !errors.Is(err, net.ErrClosed) {
		return err
	}
	return nil
}

// intent: serve the generated protobuf MangaService without custom codecs
// status: done
// next: keep proto and generated files in sync when changing the service contract
// blockers: none
// confidence: high
func (s *GRPCServer) GetManga(ctx context.Context, req *mangahubpb.GetMangaRequest) (*mangahubpb.MangaResponse, error) {
	id := strings.TrimSpace(req.GetId())
	if id == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	row := s.DB.QueryRowContext(ctx, `
SELECT id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status
FROM manga WHERE id = ?`, id)
	manga, err := scanGRPCManga(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "manga not found")
	}
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to load manga")
	}
	return &mangahubpb.MangaResponse{Manga: mangaToProto(manga)}, nil
}

func (s *GRPCServer) SearchManga(ctx context.Context, req *mangahubpb.SearchRequest) (*mangahubpb.SearchResponse, error) {
	query := "%" + strings.ToLower(strings.TrimSpace(req.GetQuery())) + "%"
	genre := strings.TrimSpace(strings.ToLower(req.GetGenre()))
	statusFilter := strings.TrimSpace(strings.ToLower(req.GetStatus()))
	limit := req.GetLimit()
	if limit <= 0 {
		limit = 24
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := s.DB.QueryContext(ctx, `
SELECT id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status
FROM manga
WHERE (lower(title) LIKE ? OR lower(author) LIKE ? OR lower(description) LIKE ?)
AND (? = '' OR lower(status) = ?)
ORDER BY title ASC
LIMIT ?`, query, query, query, statusFilter, statusFilter, limit)
	if err != nil {
		return nil, status.Error(codes.Internal, "search failed")
	}
	defer rows.Close()

	results := make([]*mangahubpb.Manga, 0)
	for rows.Next() {
		manga, err := scanGRPCManga(rows)
		if err != nil {
			return nil, status.Error(codes.Internal, "invalid manga record")
		}
		if genre == "" || hasGenre(manga.Genres, genre) {
			results = append(results, mangaToProto(manga))
		}
	}
	return &mangahubpb.SearchResponse{Results: results, Count: int32(len(results))}, nil
}

func (s *GRPCServer) UpdateProgress(ctx context.Context, req *mangahubpb.ProgressRequest) (*mangahubpb.ProgressResponse, error) {
	if req.GetToken() == "" {
		return nil, status.Error(codes.Unauthenticated, "token is required")
	}
	claims, err := s.JWT.Parse(strings.TrimPrefix(req.GetToken(), "Bearer "))
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}
	statusValue := strings.TrimSpace(req.GetStatus())
	if statusValue == "" {
		statusValue = "reading"
	}
	if req.GetMangaId() == "" || req.GetChapter() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "manga_id and positive chapter are required")
	}
	if err := validateGRPCProgress(ctx, s.DB, req.GetMangaId(), int(req.GetChapter()), statusValue); err != nil {
		return nil, err
	}
	result, err := s.DB.ExecContext(ctx, `
UPDATE user_progress
SET current_chapter = ?, status = ?, updated_at = CURRENT_TIMESTAMP
WHERE user_id = ? AND manga_id = ?`, req.GetChapter(), statusValue, claims.UserID, req.GetMangaId())
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to update progress")
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, status.Error(codes.NotFound, "manga is not in your library")
	}
	update := models.ProgressUpdate{
		UserID:    claims.UserID,
		MangaID:   req.GetMangaId(),
		Chapter:   int(req.GetChapter()),
		Timestamp: time.Now().Unix(),
	}
	count := 0
	if s.Sync != nil {
		count = s.Sync.BroadcastProgress(update)
	}
	return &mangahubpb.ProgressResponse{
		Status:         "updated",
		BroadcastCount: int32(count),
		Progress: &mangahubpb.ProgressUpdate{
			UserId:    update.UserID,
			MangaId:   update.MangaID,
			Chapter:   int32(update.Chapter),
			Timestamp: update.Timestamp,
		},
	}, nil
}

func mangaToProto(manga models.Manga) *mangahubpb.Manga {
	return &mangahubpb.Manga{
		Id:             manga.ID,
		Title:          manga.Title,
		Author:         manga.Author,
		Genres:         manga.Genres,
		Status:         manga.Status,
		TotalChapters:  int32(manga.TotalChapters),
		Description:    manga.Description,
		CoverUrl:       manga.CoverURL,
		SourceProvider: manga.SourceProvider,
		SourceUrl:      manga.SourceURL,
		RightsStatus:   manga.RightsStatus,
	}
}

type grpcScanner interface {
	Scan(dest ...any) error
}

func scanGRPCManga(scanner grpcScanner) (models.Manga, error) {
	var manga models.Manga
	var genresJSON string
	if err := scanner.Scan(
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
	); err != nil {
		return manga, err
	}
	_ = json.Unmarshal([]byte(genresJSON), &manga.Genres)
	manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
	return manga, nil
}

func hasGenre(genres []string, target string) bool {
	for _, genre := range genres {
		if strings.ToLower(genre) == target {
			return true
		}
	}
	return false
}

func validateGRPCProgress(ctx context.Context, db *database.Store, mangaID string, chapter int, statusValue string) error {
	switch statusValue {
	case "reading", "completed", "plan-to-read", "on-hold", "dropped":
	default:
		return status.Error(codes.InvalidArgument, "invalid reading status")
	}
	var total int
	if err := db.QueryRowContext(ctx, `SELECT total_chapters FROM manga WHERE id = ?`, mangaID).Scan(&total); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return status.Error(codes.NotFound, "manga not found")
		}
		return status.Error(codes.Internal, "failed to validate manga")
	}
	if total > 0 && chapter > total {
		return status.Error(codes.InvalidArgument, "chapter exceeds total chapters")
	}
	return nil
}
