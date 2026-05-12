# Demo Script

## Goal

Demonstrate the implemented MangaHub application and show exactly how it maps to the full five-protocol MangaHub requirement.

## Setup

Terminal 1:

```bash
cd services/api
go run ./cmd/server
```

Terminal 2:

```bash
npm run dev
```

Browser:

```text
http://localhost:3000
```

## Demo Flow

1. Open the landing page.
2. Point out the five protocol visual map: HTTP, TCP, UDP, gRPC, and WebSocket.
3. Open `/docs/requirements` and show the PDF-derived requirements.
4. Open `/docs/protocols` and show message shapes for TCP, UDP, gRPC, and WebSocket.
5. Open `/app`.
6. Register with `demo_user`, `demo@example.com`, and `Password123`.
7. Login if the user already exists.
8. Search for `one`.
9. Add `One Piece` to the library.
10. Update progress by one chapter.
11. Confirm that the progress update persists to the database and broadcasts to connected TCP clients.
12. Open `/docs/cli-reference` and show how the finished CLI should start and inspect all services.

## API Smoke Commands

```bash
curl http://localhost:8080/health
curl "http://localhost:8080/manga?q=one"
```

Register:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"demo_user","email":"demo@example.com","password":"Password123"}'
```

Login:

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"demo_user","password":"Password123"}'
```

## Acceptance Checklist

- Website loads at `localhost:3000`.
- Go API loads at `localhost:8080`.
- Health endpoint reports database active.
- Registration works or reports duplicate user clearly.
- Login returns a JWT.
- Manga search returns catalog manga.
- Library add succeeds with valid JWT.
- Progress update succeeds with valid JWT.
- Docs include all five protocols.
- Docs include UC-001 through UC-031.
- Docs include CLI behavior and troubleshooting.
- AI usage statement is present.

## Full Protocol Demo Checklist

- TCP client connects to `localhost:9090`.
- Progress update broadcasts to connected TCP clients.
- UDP client registers on `localhost:9091`.
- Admin-triggered notification broadcasts to UDP clients.
- gRPC client calls `GetManga`, `SearchManga`, and `UpdateProgress`.
- WebSocket client joins chat on `localhost:9093`.
- Chat message broadcasts to connected users.
