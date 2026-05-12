# MangaHub

MangaHub is a production-shaped manga tracking and reader platform built around the Net-centric Programming reference PDFs. It keeps the required protocol work visible while using the current project stack: Next.js, Go, Neon Postgres, Cloudflare R2, JWT auth, and legal metadata enrichment through the backend.

## Documentation

- [Setup Instructions](docs/setup.md)
- [API Documentation](docs/api.md)
- [Architecture Overview](docs/architecture.md)
- [Production Setup](docs/production.md)

The docs map the implementation to:

- `mangahub_project_spec (1).pdf`
- `mangahub_usecase (reference) (1).pdf`
- `mangahub_cli_manual (reference) (1).pdf`

## What Is Implemented

- Next.js product website, reader, admin UI, and docs reader in `apps/web`
- Go API and protocol services in `services/api`
- HTTP REST API for auth, manga, chapters, library, progress, admin CRUD, source enrichment, and notifications
- TCP progress sync, UDP notifications, gRPC manga/progress service, and WebSocket chat service
- Neon Postgres support through `DATABASE_URL`
- SQLite fallback for local development
- Cloudflare R2 storage for licensed uploads and app-owned media through `STORAGE_PROVIDER=r2`
- AniList GraphQL and MangaDex REST metadata enrichment through backend endpoints
- Role-gated admin manga/chapter create, update, delete, sync, and upload flows

## Media Policy

Seeded manga records are metadata-only. MangaHub does not import chapter pages from third-party scan sites and does not mirror third-party AniList or MangaDex cover images into R2. R2 is for licensed uploads and app-owned media such as chapter pages, publisher/admin cover overrides, avatars, import bundles, and generated derivatives.

The static seed preserves the status values retrieved from AniList metadata instead of manually relabeling completed titles to improve the visible mix. Use the backend AniList or MangaDex sync/admin flows to add current ongoing or hiatus titles when the demo catalog needs broader status coverage.

MangaDex cover art is resolved from the `cover_art` relationship. The API returns a cover file name, and MangaHub builds the CDN URL as `https://uploads.mangadex.org/covers/{mangaId}/{fileName}`. Chapter pages are separate from cover metadata and are only served from app-owned storage after an authorized import/upload.

## Quick Start

Install dependencies:

```bash
npm install
```

Run the Go API:

```bash
npm run api:dev
```

Run the website:

```bash
NEXT_PUBLIC_API_URL=http://localhost:8080 npm run dev --workspace apps/web -- --hostname 127.0.0.1 --port 3000
```

Open `http://127.0.0.1:3000`.

Run migrations:

```bash
npm run api:migrate
```

Test R2:

```bash
npm run api:storage-check
```

## Default Ports

- Website: `3000`
- HTTP API: `8080`
- TCP sync: `9090`
- UDP notifications: `9091`
- gRPC internal service: `9092`
- WebSocket chat: `9093`

## Service Workflows

### Role And Service Topology

```mermaid
flowchart LR
  Reader[Reader / User] --> Web[Next.js Web App]
  Admin[Admin] --> Web
  Operator[CLI / Ops Operator] --> CLI[MangaHub CLI]

  Web --> REST[HTTP REST API :8080]
  CLI --> REST
  CLI --> GRPC[gRPC Manga Service :9092]

  REST --> Auth[Auth + JWT]
  REST --> Catalog[Manga Catalog]
  REST --> Library[Bookmarks + Progress]
  REST --> AdminAPI[Admin Content APIs]
  REST --> Sources[Legal Source Adapters]
  REST --> Dispatch[Realtime Dispatch]

  Catalog --> DB[(Neon Postgres)]
  Library --> DB
  AdminAPI --> DB
  Auth --> DB

  AdminAPI --> R2[(Cloudflare R2)]
  Sources --> AniList[AniList GraphQL]
  Sources --> MangaDex[MangaDex API]

  Dispatch --> TCP[TCP Progress Sync :9090]
  Dispatch --> UDP[UDP Notifications :9091]
  Dispatch --> WS[WebSocket Chat :9093]

  TCP --> ReaderClient[CLI / Desktop Listener]
  UDP --> OpsClient[Ops Notification Listener]
  WS --> Web
```

### Reader Manga Detail

```mermaid
sequenceDiagram
  actor Reader
  participant Web as Next.js Web
  participant API as REST API
  participant DB as Neon Postgres
  participant Source as AniList / MangaDex

  Reader->>Web: Open /manga/:slug
  Web->>API: GET /manga/:slug
  API->>DB: Load catalog record
  DB-->>API: Manga metadata + source URL
  API->>Source: Optional legal metadata enrichment
  Source-->>API: Cover, tags, rankings, relations
  API-->>Web: Manga detail JSON
  Web-->>Reader: Detail page
```

### CLI And gRPC

```mermaid
sequenceDiagram
  actor Operator
  participant CLI as MangaHub CLI
  participant GRPC as gRPC Service :9092
  participant DB as Neon Postgres

  Operator->>CLI: ./mangahub grpc manga search --query "silent"
  CLI->>GRPC: SearchManga(query)
  GRPC->>DB: SELECT matching manga
  DB-->>GRPC: Result list
  GRPC-->>CLI: Protobuf SearchResponse
  CLI-->>Operator: JSON output

  Operator->>CLI: ./mangahub grpc manga get --id a-silent-voice
  CLI->>GRPC: GetManga(id)
  GRPC->>DB: SELECT exact manga ID
  DB-->>GRPC: Manga row
  GRPC-->>CLI: Protobuf MangaResponse
```

### Progress Sync Over TCP

```mermaid
sequenceDiagram
  actor Reader
  participant Web as Web App
  participant API as REST API
  participant DB as Neon Postgres
  participant TCP as TCP Sync :9090
  participant Client as Connected Sync Client

  Reader->>Web: Continue reading chapter
  Web->>API: PUT /users/progress
  API->>DB: Update user_progress
  DB-->>API: Saved
  API->>TCP: Broadcast ProgressUpdate
  TCP-->>Client: Deliver progress event
  API-->>Web: Progress saved
```

### Admin Upload And UDP Notification

```mermaid
sequenceDiagram
  actor Admin
  participant Web as Admin UI
  participant API as REST API
  participant R2 as Cloudflare R2
  participant DB as Neon Postgres
  participant UDP as UDP Notifications :9091

  Admin->>Web: Upload chapter pages
  Web->>API: POST /admin/manga/:id/chapters
  API->>R2: Store licensed page images
  R2-->>API: Public page URLs
  API->>DB: Insert chapter metadata
  DB-->>API: Saved
  API->>UDP: Fire notification event
  API-->>Web: Chapter created
```

### Protocol Responsibilities

```mermaid
flowchart TB
  HTTP[HTTP REST :8080] --> Public[Public site data]
  HTTP --> AuthFlow[Auth, library, progress]
  HTTP --> AdminFlow[Admin CRUD, sync, upload]

  GRPC[gRPC :9092] --> ServiceLookup[Service-to-service manga lookup]
  GRPC --> ServiceProgress[Structured progress updates]

  TCP[TCP :9090] --> ProgressBroadcast[Persistent progress sync channel]
  UDP[UDP :9091] --> FireForget[Fire-and-forget notifications]
  WS[WebSocket :9093] --> BrowserRealtime[Browser chat and realtime UI]

  Public --> DB[(Neon Postgres)]
  AuthFlow --> DB
  AdminFlow --> DB
  AdminFlow --> R2[(Cloudflare R2)]
```
