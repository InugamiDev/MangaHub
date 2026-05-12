package protocols

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/database"
	"mangahub/services/api/internal/models"
	mangahubpb "mangahub/services/api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type recordingProgressBroadcaster struct {
	updates []models.ProgressUpdate
}

func (b *recordingProgressBroadcaster) BroadcastProgress(update models.ProgressUpdate) int {
	b.updates = append(b.updates, update)
	return 1
}

func TestGRPCServiceUsesGeneratedProtobufClient(t *testing.T) {
	// intent: verify the required protobuf gRPC client/server path without custom codecs
	// status: done
	// next: keep this test aligned with proto/mangahub.proto when the service contract changes
	// blockers: none
	// confidence: high
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

	jwt := auth.NewJWTManager("test-secret")
	token, _, err := jwt.Generate("usr_grpc", "grpc_reader")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES (?, ?, ?, ?)`, "usr_grpc", "grpc_reader", "grpc@example.com", "hash"); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status) VALUES (?, ?, ?, ?)`, "usr_grpc", "naruto", 1, "reading"); err != nil {
		t.Fatalf("insert progress: %v", err)
	}

	listener := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	broadcaster := &recordingProgressBroadcaster{}
	mangahubpb.RegisterMangaServiceServer(grpcServer, NewGRPCServer(":0", db, jwt, broadcaster))
	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			t.Errorf("serve grpc: %v", err)
		}
	}()
	defer grpcServer.Stop()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	defer conn.Close()
	client := mangahubpb.NewMangaServiceClient(conn)

	manga, err := client.GetManga(ctx, &mangahubpb.GetMangaRequest{Id: "naruto"})
	if err != nil {
		t.Fatalf("get manga: %v", err)
	}
	if manga.GetManga().GetTitle() != "Naruto" {
		t.Fatalf("manga title = %q, want Naruto", manga.GetManga().GetTitle())
	}

	search, err := client.SearchManga(ctx, &mangahubpb.SearchRequest{Query: "naruto", Limit: 5})
	if err != nil {
		t.Fatalf("search manga: %v", err)
	}
	if search.GetCount() == 0 {
		t.Fatal("expected gRPC search results")
	}

	progress, err := client.UpdateProgress(ctx, &mangahubpb.ProgressRequest{Token: token, MangaId: "naruto", Chapter: 2, Status: "reading"})
	if err != nil {
		t.Fatalf("update progress: %v", err)
	}
	if progress.GetBroadcastCount() != 1 || len(broadcaster.updates) != 1 {
		t.Fatalf("broadcast count = %d, updates = %d", progress.GetBroadcastCount(), len(broadcaster.updates))
	}
	if progress.GetProgress().GetMangaId() != "naruto" || progress.GetProgress().GetChapter() != 2 {
		t.Fatalf("progress response = %#v", progress.GetProgress())
	}
}

func TestWebSocketOriginAllowsLocalhostLoopbackAlias(t *testing.T) {
	server := NewChatServer(":0", auth.NewJWTManager("test-secret"), "http://localhost:3000")
	if !server.originAllowed("http://127.0.0.1:3000") {
		t.Fatal("expected 127.0.0.1 dev origin to match localhost allowlist")
	}
	if server.originAllowed("http://127.0.0.1:3001") {
		t.Fatal("expected different local port to remain blocked")
	}
	if server.originAllowed("https://example.com") {
		t.Fatal("expected unrelated origin to remain blocked")
	}
}
