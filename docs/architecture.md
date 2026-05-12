# MangaHub Architecture Overview

MangaHub is implemented as a production-shaped manga tracking platform while preserving the networking requirements from the reference PDFs. The current stack is a Next.js web app plus a Go service that hosts HTTP, TCP, UDP, gRPC, and WebSocket services.

## PDF Traceability

| Reference PDF | Requirement | Current Project Fit |
| --- | --- | --- |
| `mangahub_project_spec (1).pdf` | Go backend, HTTP, TCP, UDP, gRPC, WebSocket, JWT, database, JSON manga model | Implemented in `services/api` with Gin, protocol servers, JWT auth, Neon/Postgres or SQLite, and JSON responses. |
| `mangahub_usecase (reference) (1).pdf` | UC-001 to UC-016 for users, discovery, library, progress sync, notifications, chat, gRPC | Implemented as HTTP routes and protocol services; web routes map to the user-facing flows. |
| `mangahub_cli_manual (reference) (1).pdf` | CLI command model for auth, manga, library, progress, server status, sync, chat | Implemented for the demo-critical subset; broader manual commands are recognized as local CLI stubs when no backend exists. |
| All PDFs | Manga data volume, JSON metadata, legal source integration | `services/api/data/manga.json` plus AniList/MangaDex sync cover the required metadata model without ingesting third-party page media. |

## Runtime Components

```txt
apps/web
  Next.js 15 product website, catalog, reader, auth pages, admin pages, docs reader

services/api
  Go API service
  ├── HTTP REST API on :8080
  ├── TCP progress sync on :9090
  ├── UDP notification server on :9091
  ├── gRPC manga/progress service on :9092
  └── WebSocket chat server on :9093

Neon Postgres
  Production database selected by DATABASE_URL

Cloudflare R2
  Production storage for licensed uploads/app-owned media selected by STORAGE_PROVIDER=r2

AniList GraphQL + MangaDex API
  Legal metadata enrichment through backend-only adapters
```

## Default Ports

| Component | Port | Purpose |
| --- | ---: | --- |
| Next.js web | `3000` | Product UI and docs |
| HTTP API | `8080` | REST API and auth |
| TCP sync | `9090` | Progress broadcast |
| UDP notifications | `9091` | Release notifications |
| gRPC | `9092` | Internal typed service calls |
| WebSocket | `9093` | Realtime chat |

## Request Flow

### Discovery And Detail

1. User opens a catalog/detail page in `apps/web`.
2. Web calls `GET /manga` or `GET /manga/:id`.
3. API reads MangaHub catalog metadata from Postgres.
4. If the record has an AniList or MangaDex ID in `source_url`, the web can call MangaHub's backend adapters: `GET /sources/anilist/:id` or `GET /sources/mangadex/:id`.
5. The backend calls the legal metadata API, normalizes the response, and returns metadata to the web.
6. The user remains on MangaHub. Source URLs are provenance, not outbound reading links.

### Reader Media

1. Admin creates a chapter with `POST /admin/manga/:id/chapters`.
2. If page files are uploaded, the API stores them using the configured chapter storage.
3. In production, `R2ChapterStorage` uploads licensed page files to Cloudflare R2 and stores public page URLs.
4. Reader route `/manga/[slug]/chapter/[chapterId]` loads `GET /manga/:id/chapters/:chapterId`.
5. Uploaded page URLs render in the MangaHub reader.
6. When no chapter rows exist, the API can synthesize a `source-preview` chapter from the sanitized cover URL. This preview is metadata-only and does not import third-party manga pages.

### Auth And Library

1. `POST /auth/register` creates a user with bcrypt password hash.
2. `POST /auth/login` returns a JWT.
3. The web stores the token in `localStorage` as `mangahub_token`.
4. Protected `/users/*` API routes validate the JWT.
5. Progress updates are persisted and broadcast through TCP.

### Protocol Integration

1. HTTP `/users/progress` updates the database.
2. The API constructs a `ProgressUpdate`.
3. TCP broadcaster fans the update to authenticated TCP clients.
4. If no TCP client is connected for that user, the TCP server queues up to 20 updates and replays them after the next authenticated TCP connection.
5. gRPC `UpdateProgress` uses generated protobuf code and follows the same persistence and TCP broadcast path.
6. Admin `/admin/notify` creates UDP notifications; UDP sends retry once per registered client on write failure.
7. WebSocket chat serves `/ws/chat`, validates JWTs, tracks rooms, preserves recent history, and manages browser/CLI realtime discussion channels.

## Data Model

The project started from the PDF's simplified JSON model and extends it for production metadata provenance and uploaded chapters.

### `users`

```sql
CREATE TABLE users (
  id TEXT PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  email TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'reader',
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

`ADMIN_EMAILS` promotes matching registered accounts to `admin`. Admin routes re-check the user's current database role, so role changes are not trusted only from a stale JWT claim.

### `manga`

```sql
CREATE TABLE manga (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  author TEXT NOT NULL,
  genres TEXT NOT NULL,
  status TEXT NOT NULL,
  total_chapters INTEGER NOT NULL,
  description TEXT NOT NULL,
  cover_url TEXT NOT NULL,
  source_provider TEXT NOT NULL DEFAULT 'MangaHub seed',
  source_url TEXT NOT NULL DEFAULT '',
  rights_status TEXT NOT NULL DEFAULT 'metadata-only'
);
```

`source_url` records metadata provenance, for example an AniList media URL. It should not be treated as a user-facing link.

### `chapters`

```sql
CREATE TABLE chapters (
  id TEXT PRIMARY KEY,
  manga_id TEXT NOT NULL,
  title TEXT NOT NULL,
  chapter_number REAL NOT NULL,
  page_urls TEXT NOT NULL,
  page_count INTEGER NOT NULL DEFAULT 0,
  publish_status TEXT NOT NULL DEFAULT 'draft',
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);
```

`page_urls` is JSON text. In production the values are R2 public URLs.

### `user_progress`

```sql
CREATE TABLE user_progress (
  user_id TEXT NOT NULL,
  manga_id TEXT NOT NULL,
  current_chapter INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, manga_id),
  FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
  FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);
```

## Storage Architecture

MangaHub separates metadata from media.

- Metadata: Neon Postgres in production, SQLite fallback in local/dev.
- Cover metadata: AniList GraphQL URL references or admin-supplied legal cover URL.
- App-owned media: Cloudflare R2 in production for chapter pages, publisher/admin cover overrides, avatars, import bundles, and generated derivatives.
- Local page storage: `CHAPTER_UPLOAD_DIR` only when `STORAGE_PROVIDER=local`.

R2 smoke test:

```bash
npm run api:storage-check
```

Expected:

```txt
R2 upload ok
R2 public URL ok
```

## Source Adapter Policy

The project uses legal/public metadata sources only.

- AniList GraphQL endpoint: `https://graphql.anilist.co`
- MangaDex REST endpoint: `https://api.mangadex.org`
- Backend exact lookup: `GET /sources/anilist/:id`
- Backend search fallback: `GET /sources/anilist/search`
- Backend exact lookup: `GET /sources/mangadex/:id`
- Backend search fallback: `GET /sources/mangadex/search`
- Admin imports: `POST /admin/sync/anilist` and `POST /admin/sync/mangadex`
- No manga pages are imported from AniList, MangaDex, MANGA Plus, or scanlation sites.
- AniList and MangaDex cover images are referenced as metadata URLs, not mirrored into R2.
- Admin chapter upload is the only implemented source of real reader page media; `source-preview` chapters are cover-based metadata previews.

## Security Boundaries

- User JWT protects `/users/*`.
- Admin mutation routes require a JWT whose database user has `role=admin` or the server-side `ADMIN_SYNC_TOKEN`.
- `.env` is gitignored and should have restrictive permissions.
- Inputs are validated before database writes.
- Cover URLs are sanitized to reject stale local SVG placeholders.
- Page URLs are accepted only as HTTP(S) URLs or generated `/media/chapters/*` local URLs.

## Operational Model

Local development:

```bash
npm run api:dev
npm run dev --workspace apps/web
```

Production-shaped local setup:

```bash
npm run api:migrate
npm run api:storage-check
npm run build
```

## Reliability Notes

The implementation follows the reference PDFs' realistic academic reliability targets while using production primitives:

- Neon connection pooling through the Postgres driver.
- Single-process protocol services for demo clarity.
- Graceful HTTP shutdown in the API server.
- TCP queues up to 20 undelivered progress updates per user and replays them after reconnect.
- UDP notification sends retry once per client and log failures without stopping the broadcast.
- Database migration on API boot and via `npm run api:migrate`.
- R2 upload errors return explicit API failures instead of silently falling back.
- Full process restart and browser-side automatic WebSocket reconnect remain demo/ops responsibilities rather than a multi-instance recovery system.
