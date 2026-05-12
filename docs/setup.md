# MangaHub Setup Instructions

This guide is the operational setup for the current MangaHub project. It is tied to the provided reference PDFs but reflects the actual repository structure, Neon database, R2 storage, and Next.js web app.

## What This Sets Up

From the PDFs, MangaHub needs:

- Go backend with HTTP, TCP, UDP, gRPC, and WebSocket.
- JWT auth.
- Manga catalog and user progress database.
- Manga discovery, detail, library, progress, notifications, chat, and gRPC services.
- CLI-compatible server behavior and health/status commands.

In this repository, those requirements map to:

- `services/api`: Go backend and protocol services.
- `apps/web`: Next.js web app, reader, admin UI, and docs.
- Neon Postgres: production database via `DATABASE_URL`.
- Cloudflare R2: production storage for licensed uploads and app-owned media.
- AniList GraphQL and MangaDex API: legal metadata enrichment, called only by the backend.

## Prerequisites

Install:

- Node.js and npm.
- Go 1.26, matching `services/api/go.mod`.
- A Neon Postgres database.
- A Cloudflare R2 bucket for chapter pages. `.env.example` uses `mangahub-chapter-pages`.

Verify:

```bash
node --version
npm --version
go version
```

## Install Dependencies

From the repository root:

```bash
npm install
```

Go dependencies are resolved by Go modules:

```bash
cd services/api
go mod tidy
```

## Environment File

Create `.env` from the example:

```bash
cp .env.example .env
chmod 600 .env
```

Required values:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080

PORT=8080
TCP_PORT=9090
UDP_PORT=9091
GRPC_PORT=9092
WEBSOCKET_PORT=9093
ALLOWED_ORIGIN=http://localhost:3000

JWT_SECRET=<generated-long-secret>
ADMIN_EMAILS=admin@example.com
ADMIN_SYNC_TOKEN=<generated-long-admin-token>
ANILIST_BOOTSTRAP_STATUSES=RELEASING,HIATUS
ANILIST_BOOTSTRAP_LIMIT=30
MANGADEX_BOOTSTRAP_STATUSES=ongoing,hiatus
MANGADEX_BOOTSTRAP_LIMIT=50

DATABASE_URL=postgresql://<user>:<password>@<host>/<db>?sslmode=require&channel_binding=require
DATABASE_PATH=mangahub.db

STORAGE_PROVIDER=r2
R2_ACCOUNT_ID=<cloudflare-account-id>
R2_ENDPOINT=
R2_BUCKET=mangahub-chapter-pages
R2_ACCESS_KEY_ID=<r2-access-key-id>
R2_SECRET_ACCESS_KEY=<r2-secret-access-key>
R2_REGION=auto
R2_PUBLIC_BASE_URL=https://<public-r2-domain>
```

Generate app-owned secrets:

```bash
openssl rand -hex 64
openssl rand -base64 48
```

Do not commit `.env`.

## Database Migration

Run schema migration and seed upsert against Neon:

```bash
npm run api:migrate
```

Expected:

```txt
database migration complete: dialect=postgres
```

If `DATABASE_URL` is empty, the API uses SQLite at `services/api/mangahub.db`; this is only a local fallback.

## R2 Storage Test

Test the configured R2 bucket:

```bash
npm run api:storage-check
```

Expected:

```txt
R2 upload ok
R2 public URL ok
```

This writes a tiny smoke-test PNG through the same storage adapter used by chapter uploads. R2 is reserved for media MangaHub owns or is licensed to host: chapter pages, publisher/admin cover overrides, avatars, import bundles, and generated derivatives. AniList and MangaDex cover URLs remain third-party metadata references and should not be copied into R2.

## Start Development Servers

Terminal 1:

```bash
npm run api:dev
```

Starts:

- HTTP API: `http://localhost:8080`
- TCP sync: `:9090`
- UDP notifications: `:9091`
- gRPC: `:9092`
- WebSocket chat: `:9093`

Terminal 2:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev --workspace apps/web -- --hostname 127.0.0.1 --port 3000
```

Open:

```txt
http://127.0.0.1:3000
```

## Admin Access

Admin pages require either a signed-in user with `role=admin` or `ADMIN_SYNC_TOKEN`.

1. Set `ADMIN_EMAILS` to the email address that should become admin.
2. Register and login with that email, or paste `ADMIN_SYNC_TOKEN` into the optional server-token field.
3. Open `http://127.0.0.1:3000/admin/manga`.
4. Uploads and catalog mutations from the web UI are sent with the admin JWT first; the sync token remains a server-to-server fallback.
5. CLI admin commands use `MANGAHUB_ADMIN_TOKEN` or `ADMIN_SYNC_TOKEN` as `X-Admin-Sync-Token`; they do not currently reuse the saved user JWT for admin mutations.

## Core Manual Test Flow

This maps to the CLI manual quick start and use cases UC-001 through UC-006.

1. Register at `/register`.
2. Login at `/login`.
3. Browse `/library`.
4. Open a manga detail page such as `/manga/attack-on-titan`.
5. Confirm metadata is displayed inside MangaHub from backend/AniList enrichment.
6. Add or manage manga in admin.
7. Upload a chapter in `/admin/manga/:id/chapters/new`.
8. Open `/manga/:id/chapter/:chapterId` and confirm uploaded R2 pages render.

If a manga has no uploaded chapter rows, the API may expose a `source-preview` chapter backed by the sanitized cover URL. This is a safe metadata preview and not third-party page ingestion.

The bundled seed preserves AniList status values as imported. It is currently completed-heavy, so do not manually flip completed titles to ongoing or hiatus for presentation. Use backend AniList sync/admin import to add current ongoing or hiatus titles when the catalog needs that status mix.

## API Smoke Checks

```bash
curl -I http://127.0.0.1:3000
curl http://localhost:8080/health
curl http://localhost:8080/manga/attack-on-titan
curl http://localhost:8080/sources/anilist/53390
```

Note: in some local sandbox shells, `curl` may fail to connect to the API even while the browser and Next.js server can reach it. Use the server logs and browser smoke test as the fallback signal.

## Build And Test

Run backend tests:

```bash
npm run api:test
```

Run frontend lint:

```bash
npm run lint --workspace apps/web
```

Run frontend production build:

```bash
npm run build --workspace apps/web
```

Run all high-signal checks:

```bash
npm run verify
```

## CLI Mapping

The CLI manual describes commands such as:

- `mangahub server start`
- `mangahub auth register`
- `mangahub auth login`
- `mangahub manga search`
- `mangahub library add`
- `mangahub progress update`
- `mangahub server health`

In this repository, the equivalent developer commands are:

| CLI Manual Concept | Current Repo Command Or Route |
| --- | --- |
| Start all servers | `npm run api:dev` |
| Start web UI | `npm run dev --workspace apps/web` |
| Register | `POST /auth/register` or `/register` |
| Login | `POST /auth/login` or `/login` |
| Search manga | `GET /manga?q=...` or `/search?q=...` |
| Add library item | `POST /users/library` |
| Update progress | `PUT /users/progress` |
| Server health | `GET /health` |
| Storage check | `npm run api:storage-check` |
| CLI test console from repo root | `npm run api:console` |
| CLI test console from backend dir | `cd services/api && go run ./cmd/mangahub test console --json` |
| gRPC manga lookup | `cd services/api && go run ./cmd/mangahub grpc manga get --id attack-on-titan` |
| Admin catalog CRUD | `/admin/manga`, `/admin/manga/:id/chapters`, or `mangahub admin ...` with `ADMIN_SYNC_TOKEN` |
| Source metadata | `GET /sources/anilist/:id`, `GET /sources/mangadex/:id`, `/admin/sync/anilist`, `/admin/sync/mangadex` |

The local CLI recognizes the broader manual command surface. Implemented commands include `init`, `version`, `auth`, `manga`, `library`, `progress`, `admin`, `test console`, `sync monitor`, `notify subscribe`, `chat send`, `grpc`, `server health/status`, `config show/set/reset`, `profile list`, and `export library`. Manual commands without backend support, such as `server start`, `server stop`, `backup`, most `db`, `logs`, and `update` operations, return a clear local-CLI-not-implemented error instead of an unknown command.

## Troubleshooting

### API cannot connect to Neon

Check:

- `DATABASE_URL` host, password, and database name.
- `sslmode=require`.
- Network/DNS access.

Then rerun:

```bash
npm run api:migrate
```

### R2 upload fails

Check:

- `STORAGE_PROVIDER=r2`
- `R2_BUCKET=mangahub-chapter-pages` or your configured bucket name
- R2 access key has object write permission.
- `R2_PUBLIC_BASE_URL` points to a public bucket/custom domain.

Then rerun:

```bash
npm run api:storage-check
```

### Admin actions fail

Check:

- `ADMIN_EMAILS` includes the account email when using admin JWT auth.
- `ADMIN_SYNC_TOKEN` in `.env` when using the server-token fallback or CLI admin commands.
- Admin token field in the UI when using the server-token fallback.
- API server logs for `401`, `403`, or `503`.

### Reader has no pages

Seeded manga records carry metadata only. Until a licensed chapter upload exists, MangaHub may show a `source-preview` chapter generated from the sanitized cover URL. Upload pages through admin to create real reader media.
