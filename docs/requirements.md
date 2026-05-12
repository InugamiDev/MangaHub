# MangaHub Requirements

## Project Identity

MangaHub is a manga and comic tracking system for the Network Programming / Net-centric Programming course. The implementation language is Go. The project is intended for a two-student team over roughly 10-12 weeks.

## Objectives

- Build practical experience with Go network application development.
- Demonstrate all five required communication protocols: HTTP, TCP, UDP, gRPC, and WebSocket.
- Strengthen networking concepts through a manageable manga tracking product.
- Practice concurrency with goroutines and basic distributed-system patterns.
- Produce a working demo that proves the protocols work together.

## Deliverables

- Source code and documentation submitted on Blackboard before the due date.
- Zip naming format: `GroupXX_MangaHub.zip`, for example `Group01_MangaHub.zip`.
- Final submission deadline: `23:59` on demo day.
- Live demonstration at the end of the course.
- Missing the demo session results in zero project grade.

## Data Requirements

- Manual entry: 100 popular manga series with essential metadata.
- API integration: 100 additional series from MangaDx or another legal API.
- Educational scraping only from practice sites such as `quotes.toscrape.com` or `httpbin.org`.
- Store manga data in JSON for simplicity.
- Include at least 30-40 different manga series across major genres.
- Include at least 15-20 series per major genre such as shounen, shoujo, seinen, and josei.
- Required metadata: `id`, `title`, `author`, `genres`, `status`, `total_chapters`, `description`, `cover_url`.

## Core Functional Requirements

- Register users with username, email, and password.
- Hash passwords with bcrypt.
- Authenticate users and return JWT tokens.
- Search manga by title or author with optional status and genre filters.
- View manga details.
- Add manga to a user library.
- Update reading progress.
- Broadcast progress updates through TCP in the protocol phase.
- Register clients and broadcast chapter notifications through UDP in the protocol phase.
- Support real-time chat through WebSocket in the protocol phase.
- Expose internal manga and progress operations through gRPC in the protocol phase.

## Core Services

- HTTP REST API: authentication, manga search/details, library, and progress.
- TCP progress sync server: concurrent client connections and JSON progress messages.
- UDP notification server: registration and broadcast notifications.
- WebSocket chat hub: join, leave, and real-time message broadcast.
- gRPC internal service: `GetManga`, `SearchManga`, and `UpdateProgress`.
- Database layer: SQLite persistence for users, manga, and progress.

## Performance Targets

- Support 50-100 concurrent users during testing.
- Handle at least 30-40 manga series in the database.
- Return basic search queries within 500ms.
- Support 20-30 concurrent TCP connections.
- Support WebSocket chat with 10-20 simultaneous users.
- Maintain 80-90% uptime during the demonstration period.
- Generate authentication tokens in less than 100ms.
- Broadcast progress updates within 1 second.
- Deliver chat messages within 100ms.
- Complete simple database operations within 200ms.

## Minimum Passing Requirements

- All five network protocols implemented and functional.
- Basic user authentication and authorization.
- Manga data storage and retrieval.
- Reading progress tracking and synchronization.
- Real-time chat functionality.
- Successful live demonstration.

## Current Implementation Audit

This matrix compares the three reference PDFs with the current repository. It is intentionally scoped to the grading-critical requirements and the CLI/web behavior that can be verified in `services/api` and `apps/web`.

| Area | PDF Requirement | Current Status | Evidence |
| --- | --- | --- | --- |
| HTTP REST API | Register, login, search/detail, library add/list, progress update, JSON errors, CORS | Implemented | `services/api/internal/handlers/server.go`; covered by `TestHTTPApplicationFlow`. |
| Auth and security | bcrypt password hashes, JWT protected user routes, input validation | Implemented | `internal/auth`, JWT middleware, registration/library validation tests. |
| Database layer | Users, manga, user progress in SQLite; JSON manga fields | Implemented with production extension | SQLite fallback plus Postgres via `DATABASE_URL`; `genres` and chapter pages stored as JSON text. |
| Manga data volume | 100 manual + 100 API records, at least 30-40 series, 15-20 per major genre | Implemented with legal metadata seed plus AniList bootstrap | Current seed file has 201 AniList metadata records, including Solo Leveling, with 50 records each tagged Shounen, Shoujo, Seinen, and Josei. Covers use the highest AniList URL verified for each title; some records legitimately use `medium` because no `large` object exists. The static seed is completed-heavy because it preserves imported AniList statuses; use `/admin/sync/anilist` or `ANILIST_BOOTSTRAP_STATUSES=RELEASING,HIATUS` to add current ongoing or hiatus titles rather than relabeling completed records. |
| Legal/source integration | Simple MangaDex or other legal API metadata integration | Implemented with AniList and MangaDex | `/sources/anilist/:id`, `/sources/anilist/search`, `/sources/mangadex/:id`, `/sources/mangadex/search`, `/admin/sync/anilist`, and `/admin/sync/mangadex`. |
| TCP progress sync | Authenticated TCP clients, concurrent connections, JSON progress broadcast | Implemented | `internal/protocols/tcp.go`; HTTP and gRPC progress updates call `BroadcastProgress`. |
| UDP notifications | Client registration and chapter notification broadcast | Implemented | `internal/protocols/udp.go`; admin trigger at `POST /admin/notify`. |
| WebSocket chat | Join, leave, recent history, message broadcast, length validation | Implemented | `internal/protocols/websocket.go`; CLI `chat send`. |
| gRPC service | `GetManga`, `SearchManga`, `UpdateProgress` unary RPCs | Implemented | `internal/protocols/grpc.go`; `proto/mangahub.proto`; CLI `grpc` commands. |
| CLI quick-start subset | init/version/auth/search/info/library/progress/sync/notify/chat/grpc/health | Partial | Implemented in `cmd/mangahub`; server health/status, config show/set/reset, profile list, export library, and demo protocol commands are shipped. Unsupported manual commands such as server start/stop/logs, backup, most db operations, and update are recognized stubs with clear local-CLI-not-implemented errors. |
| Web app | Optional/simple UI for discovery, auth, library, reader, admin | Implemented beyond PDF minimum | `apps/web` routes cover catalog, auth, library, reader, admin, and docs. |
| Chapter reader media | Reader pages and admin upload | Implemented extension | Admin chapter CRUD and upload storage via local files or R2. R2 is for licensed uploads/app-owned media such as chapter pages, publisher/admin cover overrides, avatars, import bundles, and generated derivatives; it is not a mirror for third-party AniList or MangaDex covers. |
| Performance targets | 50-100 users, <500ms search, 20-30 TCP clients, 10-20 chat users | Partial / unproven | Basic indexes and connection handling exist; no load-test evidence is committed. |
| Recovery use cases | DB cache/queue, TCP restart/queue, WebSocket client reconnect | Partial / not guaranteed | TCP queues up to 20 offline progress updates per user and replays them after auth reconnect; broader DB cache/restart recovery and browser WebSocket reconnect remain limited. |
| Bonus features | reviews, friends, stats, Redis cache, exports, streaming gRPC, delivery confirmation | Mostly missing | Not in current backend/web implementation except limited catalog metadata enrichment and admin extensions. |

## Grading Notes

The PDFs contain overlapping point sections. Treat the following as the practical grading intent:

- Core protocol implementation is the highest priority.
- System integration and architecture must prove the services work together.
- Code quality, tests, documentation, and demo readiness are required.
- Bonus features can add points but should not replace the five-protocol baseline.

## Bonus Feature Areas

- Enhanced TCP conflict resolution.
- WebSocket room management.
- UDP delivery confirmation.
- gRPC streaming.
- Advanced search and filtering.
- Redis caching.
- Recommendation system.
- Reviews and ratings.
- Friend system and activity feed.
- Reading statistics.
- Notification preferences.
- Data export/import.
- API versioning and OpenAPI docs.
- Health checks and graceful shutdown.
- Docker Compose, CI/CD, migrations, monitoring, and alerting.
