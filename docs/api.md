# MangaHub API Documentation

This document describes the implemented MangaHub API in `services/api`. It maps the HTTP, protocol, auth, library, admin, and metadata flows from the three provided reference PDFs to the current production code.

Reference alignment:

- `mangahub_project_spec (1).pdf`: HTTP REST API, JWT auth, manga search/detail, user library, progress sync, TCP, UDP, gRPC, WebSocket, JSON data model.
- `mangahub_usecase (reference) (1).pdf`: UC-001 through UC-016 for registration, authentication, discovery, library, progress, TCP, UDP, WebSocket, and gRPC.
- `mangahub_cli_manual (reference) (1).pdf`: command behavior for auth, manga search/info, library add/remove, progress update, server health, protocol status, and JSON output.

Data traceability:

| PDF requirement | Current implementation |
| --- | --- |
| JSON manga storage with at least 30-40 manga and 15-20 per major genre | `services/api/data/manga.json` is the source seed and currently exceeds the minimum catalog size and major-genre coverage. |
| 100 manual plus 100 legal API/imported records | The repo uses a legal metadata seed plus backend AniList and MangaDex sync paths instead of scraping manga pages. |
| Metadata fields: `id`, `title`, `author`, `genres`, `status`, `total_chapters`, `description`, `cover_url` | The shared manga DTO includes those fields plus source provenance and rights metadata. |

## Base URLs

Local development:

```txt
Web:  http://127.0.0.1:3000
API:  http://localhost:8080
```

The web app reads `NEXT_PUBLIC_API_URL`. The Go API reads `PORT`, `ALLOWED_ORIGIN`, `DATABASE_URL`, `JWT_SECRET`, `ADMIN_EMAILS`, `ADMIN_SYNC_TOKEN`, source bootstrap variables, and storage variables from `.env`.

## Auth Model

User routes use JWT bearer tokens returned by `POST /auth/login`. Tokens include a `role` claim.

```http
Authorization: Bearer <jwt>
```

Admin mutation routes accept either an authenticated user whose database role is `admin`:

```http
Authorization: Bearer <admin-user-jwt>
```

or the server-to-server admin sync token:

```http
X-Admin-Sync-Token: <ADMIN_SYNC_TOKEN>
```

## Media And Source Policy

MangaHub does not import manga pages from third-party scan sites.

- AniList and MangaDex are used as legal metadata sources through the backend.
- The backend calls AniList GraphQL at `https://graphql.anilist.co`.
- The backend calls the MangaDex REST API at `https://api.mangadex.org`.
- `source_url` is stored as provenance/input for metadata enrichment. It is not rendered as a user-facing outbound CTA.
- Cover images are metadata URL references from legal source APIs where available.
- R2 stores licensed uploads and app-owned media only: chapter pages, publisher/admin cover overrides, avatars, import bundles, and generated derivatives.
- AniList and MangaDex cover images are not mirrored into R2.
- Reader page images come from admin-uploaded licensed files stored in R2 or local chapter storage.
- If no chapter rows exist, the backend returns a controlled `source-preview` chapter using the sanitized cover URL as metadata-only preview media. This is not a mirrored manga chapter and does not import third-party page content.

## Health

### `GET /health`

Returns service and database status.

Response:

```json
{
  "status": "healthy",
  "database": {
    "status": "active",
    "dialect": "postgres"
  },
  "services": {
    "http": "online",
    "tcp": "online",
    "udp": "online",
    "grpc": "online",
    "websocket": "online"
  }
}
```

Protocol services report their configured runtime state. Expected values include `not_configured`, `configured`, `online`, `stopped`, and `error`.

PDF traceability: project spec HTTP service health; CLI manual `mangahub server health` and `mangahub server status`.

## Authentication

### `POST /auth/register`

Creates a user account. Implements UC-001.

Request:

```json
{
  "username": "demo_user",
  "email": "demo@example.com",
  "password": "Password123"
}
```

Validation:

- `username`: 3-32 characters, letters/numbers/underscore only.
- `email`: valid email format.
- `password`: at least 8 characters.
- Username and email must be unique.

Success: `201 Created`

```json
{
  "user": {
    "id": "usr_...",
    "username": "demo_user",
    "email": "demo@example.com",
    "role": "reader"
  }
}
```

If the account email is listed in `ADMIN_EMAILS`, the created user is promoted to `admin`.

### `POST /auth/login`

Authenticates a user and returns a JWT. Implements UC-002.

Request with username:

```json
{
  "username": "demo_user",
  "password": "Password123"
}
```

Request with email:

```json
{
  "email": "demo@example.com",
  "password": "Password123"
}
```

Success:

```json
{
  "token": "jwt-token",
  "expires_at": "2026-05-12T10:30:00Z",
  "user": {
    "id": "usr_...",
    "username": "demo_user",
    "email": "demo@example.com",
    "role": "reader"
  }
}
```

### `GET /users/me`

Returns the authenticated user profile, including `role`. Requires a user JWT.

### `POST /auth/recovery/request`

Starts the local demo password recovery flow.

Request:

```json
{
  "email": "demo@example.com"
}
```

Response returns a one-hour `token` in local/demo mode so the flow can be verified without email infrastructure. Production deployments should send the token by email instead of displaying it.

### `POST /auth/recovery/reset`

Consumes a recovery token and sets a new password.

```json
{
  "token": "rst_...",
  "new_password": "Password456"
}
```

## Manga Catalog

### `GET /manga`

Searches manga by title, author, or description. Implements UC-003.

Query parameters:

| Name | Required | Notes |
| --- | --- | --- |
| `q` | No | Search text. Empty query lists catalog records. |
| `genre` | No | Exact genre filter after result load. |
| `status` | No | Exact status filter, e.g. `ongoing`, `completed`, `hiatus`. |
| `limit` | No | Integer 1-500 for HTTP catalog search. Defaults to 24. gRPC search caps at 100. |
| `offset` | No | Integer 0-10000 for paginated HTTP catalog search. |
| `min_rating` | No | Community review average threshold from `0` to `5`. |
| `year` | No | Exact `publication_year` filter for imported/admin records that include year metadata. |
| `sort` | No | `title`, `rating`, `popular`, `chapters`, or `year`. |

Example:

```bash
curl "http://localhost:8080/manga?q=one&limit=5"
```

Response:

```json
{
  "results": [
    {
      "id": "one-piece",
      "title": "One Piece",
      "author": "Oda Eiichiro",
      "genres": ["Action", "Adventure", "Shounen"],
      "status": "ongoing",
      "total_chapters": 1100,
      "description": "A young pirate's adventure...",
      "cover_url": "https://s4.anilist.co/file/anilistcdn/media/manga/cover/large/...",
      "source_provider": "AniList metadata",
      "source_url": "https://anilist.co/manga/30013",
      "rights_status": "metadata-only",
      "publication_year": 1997,
      "average_rating": 4.8,
      "review_count": 12
    }
  ],
  "count": 1
}
```

### `GET /manga/:id`

Returns one manga record. Implements UC-004.

Example:

```bash
curl "http://localhost:8080/manga/attack-on-titan"
```

## Chapters

Chapter metadata and page URLs are first-class backend data. Page URLs are generated by the storage layer during admin upload. If a manga has no stored chapter rows, the API may synthesize a `source-preview` chapter from the sanitized cover URL so the reader can show a safe preview instead of failing.

### `GET /manga/:id/chapters`

Returns all chapters for a manga.

Response:

```json
{
  "chapters": [
    {
      "id": "chapter-1",
      "manga_id": "one-piece",
      "title": "Opening",
      "chapter_number": 1,
      "page_urls": ["https://cdn.example.com/chapters/one-piece/chapter-1/page.png"],
      "page_count": 1,
      "publish_status": "published",
      "updated_at": "2026-05-12T05:55:37Z"
    }
  ],
  "count": 1
}
```

### `GET /manga/:id/chapters/:chapterId`

Returns one chapter. `chapterId` may be the chapter row ID. The backend also supports numeric lookup for old progress links like `/chapter/1`.

## User Library

All routes in this section require a user JWT.

### `POST /users/library`

Adds or updates a manga in the authenticated user's library. Implements UC-005.

Request:

```json
{
  "manga_id": "one-piece",
  "status": "reading",
  "current_chapter": 1
}
```

Allowed statuses:

- `reading`
- `completed`
- `plan-to-read`
- `on-hold`
- `dropped`

### `GET /users/library`

Returns library entries joined with manga metadata.

### `DELETE /users/library/:mangaID`

Removes a manga from the authenticated user's library.

### `PUT /users/progress`

Updates reading progress for a manga already in the user's library. Implements UC-006 and triggers TCP progress fanout.

Request:

```json
{
  "manga_id": "one-piece",
  "chapter": 1095,
  "status": "reading"
}
```

Response:

```json
{
  "status": "updated",
  "sync": {
    "local_database": "updated",
    "tcp_broadcast": "sent",
    "tcp_broadcast_count": 1,
    "tcp_queue_count": 0
  },
  "progress": {
    "user_id": "usr_...",
    "manga_id": "one-piece",
    "chapter": 1095,
    "timestamp": 1778540322
  }
}
```

`tcp_broadcast` values:

| Value | Meaning |
| --- | --- |
| `sent` | At least the TCP broadcaster was called for the update. |
| `queued` | No active TCP client received the update, so the TCP server queued it for replay. |
| `no_tcp_clients` | No active TCP client was available and no queue reporter was attached. |
| `not_queued_no_broadcaster` | The HTTP server was started without a TCP broadcaster. |

The TCP server keeps up to 20 pending progress updates per user and replays them after that user's next authenticated TCP connection. This implements the UC-006/UC-026 local-update-plus-queued-broadcast behavior at the single-process service level.

### `GET /users/stats`

Returns personal reading statistics for UC-022 and UC-023: library count, completed/reading counts, total chapters read, average review rating, favorite genres, status breakdown, and per-day trend points.

### `POST /manga/:id/reviews`

Submits or updates the authenticated user's review for a completed manga. Implements UC-018.

Rules:

- User must have the manga in library with status `completed`, or have progress at/above total chapters.
- `rating` must be 1-5.
- `body` must be 5-2000 characters.

### `GET /manga/:id/reviews`

Returns public community reviews plus `average_rating` and `review_count`. Implements UC-019.

### `POST /users/friends/request`

Sends a friend request by `user_id`, `username`, or `email`. Implements the request half of UC-020.

```json
{
  "username": "friend_reader"
}
```

### `POST /users/friends/respond`

Accepts or declines an inbound friend request. Implements the approval half of UC-020.

```json
{
  "requester_id": "usr_...",
  "status": "accepted"
}
```

### `GET /users/friends`

Lists accepted friends plus inbound and outbound pending requests.

### `GET /users/activity`

Returns recent accepted-friend completions and reviews. Implements UC-021.

## Legal Metadata Sources

### `GET /sources`

Lists configured source adapters.

Current adapters:

- Internal MangaHub catalog: `/manga`
- AniList GraphQL exact lookup: `/sources/anilist/:id`
- MangaDex exact lookup: `/sources/mangadex/:id`
- MANGA Plus reference: link-out reference only, not page ingestion

### `GET /sources/anilist/:id`

Fetches exact manga metadata from AniList GraphQL by AniList media ID. This is the preferred enrichment path when a catalog record stores an AniList media URL.

Example:

```bash
curl "http://localhost:8080/sources/anilist/53390"
```

Response:

```json
{
  "result": {
    "id": "anilist-53390",
    "title": "Attack on Titan",
    "author": "Hajime Isayama",
    "genres": ["Action", "Drama", "Fantasy"],
    "status": "completed",
    "total_chapters": 139,
    "description": "...",
    "cover_url": "https://...",
    "source_provider": "AniList GraphQL",
    "source_url": "https://anilist.co/manga/53390",
    "rights_status": "metadata-only"
  },
  "source": {
    "provider": "AniList GraphQL",
    "rights_status": "metadata-only"
  }
}
```

### `GET /sources/anilist/search?q=:query&limit=:n&status=:status`

Search fallback for admin/source discovery. The UI should prefer exact ID enrichment when a catalog `source_url` contains an AniList manga ID.

Supported statuses include `completed`, `ongoing`, `hiatus`, `cancelled`, `not-yet-released`, and native AniList status values. `limit` is capped at 50.

### `GET /sources/mangadex/:id`

Fetches exact manga metadata from MangaDex by UUID. Catalog records store `source_url` as `https://mangadex.org/title/<uuid>` for provenance.

### `GET /sources/mangadex/search?q=:query&limit=:n&status=:status`

Search fallback for MangaDex metadata. It maps MangaDex title, tags, status, author/artist, and cover relationship data into the shared MangaHub manga DTO.

Supported statuses are `completed`, `ongoing`, `hiatus`, and `cancelled`. `limit` is capped at 100.

### `POST /admin/sync/anilist`

Imports metadata-only manga records from AniList. Requires an admin JWT or `ADMIN_SYNC_TOKEN`.

Request:

```json
{
  "query": "one piece",
  "status": "RELEASING",
  "statuses": ["RELEASING", "HIATUS"],
  "limit": 25
}
```

`status` is appended to `statuses` when both are supplied. If neither is supplied, sync searches all supported AniList status buckets.

### `POST /admin/sync/mangadex`

Imports metadata-only manga records from MangaDex. Requires an admin JWT or `ADMIN_SYNC_TOKEN`.

Request:

```json
{
  "query": "one piece",
  "status": "ongoing",
  "statuses": ["ongoing", "hiatus"],
  "limit": 50
}
```

`status` is appended to `statuses` when both are supplied. If neither is supplied, sync searches MangaDex `completed`, `ongoing`, `hiatus`, and `cancelled` buckets.

## Admin Manga Management

All `/admin/*` mutation routes require either an authenticated `admin` user JWT or `ADMIN_SYNC_TOKEN`.

### `POST /admin/manga`

Creates a manga record.

Request:

```json
{
  "id": "solo-leveling",
  "title": "Solo Leveling",
  "author": "Chugong",
  "genres": ["Action", "Fantasy"],
  "status": "completed",
  "total_chapters": 201,
  "description": "Hunter progression story.",
  "cover_url": "https://example.com/cover.jpg",
  "source_provider": "Admin",
  "source_url": "",
  "rights_status": "licensed-upload",
  "publication_year": 2016
}
```

### `PUT /admin/manga/:id`

Updates manga metadata.

### `DELETE /admin/manga/:id`

Deletes a manga and cascades related chapter/progress records where supported by the database.

## Admin Chapter Management

### `POST /admin/manga/:id/chapters`

Creates a chapter. Supports JSON page URLs or multipart page uploads.

JSON request:

```json
{
  "id": "chapter-1",
  "title": "Opening",
  "chapter_number": 1,
  "page_urls": ["https://cdn.example.com/page-1.png"],
  "publish_status": "published"
}
```

Multipart fields:

| Field | Required | Notes |
| --- | --- | --- |
| `title` | Yes | Chapter title. |
| `number` or `chapter_number` | Yes | Numeric chapter number. |
| `publish_status` | No | `draft`, `published`, or `scheduled`. |
| `pages` | Yes for upload | One or more PNG/JPG/WebP/GIF files. |

Storage:

- `STORAGE_PROVIDER=r2`: uploads page files to Cloudflare R2 and stores public URLs.
- `STORAGE_PROVIDER=local`: stores files in `CHAPTER_UPLOAD_DIR` and serves `/media/chapters/*`.

Do not use R2 as a cache or mirror for third-party AniList or MangaDex covers. Store only app-owned or licensed media there.

### `PUT /admin/manga/:id/chapters/:chapterId`

Updates chapter title, number, status, and page URL metadata.

### `DELETE /admin/manga/:id/chapters/:chapterId`

Deletes a chapter row.

## Admin Notifications

### `POST /admin/notify`

Broadcasts a notification to registered UDP clients. Implements UC-010.

Request:

```json
{
  "manga_id": "one-piece",
  "message": "Chapter 1096 is available",
  "type": "chapter_release"
}
```

## Protocol Services

### TCP Progress Sync `:9090`

Implements UC-007 and UC-008. Clients authenticate with a newline-delimited JSON message:

```json
{"type":"auth","token":"<jwt>"}
```

Progress updates are broadcast as JSON:

```json
{"user_id":"usr_...","manga_id":"one-piece","chapter":1095,"timestamp":1778540322}
```

If no authenticated TCP client is connected for the user, the server queues up to 20 updates and replays them after the next successful TCP auth handshake.

### UDP Notifications `:9091`

Implements UC-009 and UC-010. UDP clients register to receive notification payloads broadcast by `/admin/notify`.

Supported client messages are JSON packets with `type` equal to `register`, `subscribe`, `unregister`, `unsubscribe`, or `ping`. Broadcast sends retry once per client on write failure and log the failed address if the retry also fails.

### gRPC `:9092`

Implements UC-014 through UC-016. Proto schema is in `services/api/proto/mangahub.proto`.

The Go server and CLI use generated protobuf code in `services/api/proto/mangahub.pb.go` and `services/api/proto/mangahub_grpc.pb.go` for wire-compatible gRPC calls.

Methods:

- `GetManga`
- `SearchManga`, capped at 100 results.
- `UpdateProgress`, requiring `token`, `manga_id`, a positive `chapter`, and optional `status`. Successful progress updates persist to the database and trigger TCP broadcast or queue behavior.

### WebSocket Chat `:9093`

Implements UC-011 through UC-013. Browser/CLI clients connect for real-time chat, join/leave events, and message fanout.

Endpoint and auth:

```txt
ws://localhost:9093/ws/chat?token=<jwt>&room=global
```

Clients may also send the first message as `{"type":"join","token":"<jwt>","room":"global"}`. Use `manga_id` or room `manga:<id>` for manga-specific rooms. The server keeps the last 30 messages per room and rejects messages longer than 1000 characters.

## CLI Test Console

The backend-owned CLI includes a metrics console for demo verification:

```bash
cd services/api
go run ./cmd/mangahub test console --json
npm run api:console
```

It reports status and latency for HTTP health, HTTP manga search, gRPC search, UDP ping, TCP authenticated ping, and WebSocket authenticated ping. TCP and WebSocket checks require a saved user JWT from `mangahub auth login`; without a token they are reported as skipped.

## Error Shape

Errors are JSON where possible:

```json
{
  "error": "manga not found"
}
```

Common status codes:

- `400`: invalid input
- `401`: invalid or missing token
- `403`: authenticated but not authorized, or invalid admin sync token
- `404`: record not found
- `409`: conflict
- `500`: server error
- `502`: upstream metadata provider failure
- `503`: required service configuration is missing or a service is unavailable
