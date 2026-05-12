# Production Setup

MangaHub uses Neon Postgres for production data and Cloudflare R2 for licensed uploads/app-owned media. Local SQLite and local uploads are development fallbacks only.

## Required Environment

```env
NEXT_PUBLIC_API_URL=https://api.example.com

PORT=8080
TCP_PORT=9090
UDP_PORT=9091
GRPC_PORT=9092
WEBSOCKET_PORT=9093
ALLOWED_ORIGIN=https://mangahub.example.com

JWT_SECRET=<long-random-secret>
ADMIN_SYNC_TOKEN=<long-random-admin-token>

DATABASE_URL=postgresql://<user>:<password>@<neon-host>/<db>?sslmode=require&channel_binding=require

STORAGE_PROVIDER=r2
R2_ACCOUNT_ID=<cloudflare-account-id>
R2_BUCKET=mangahub-chapter-pages
R2_ACCESS_KEY_ID=<r2-access-key-id>
R2_SECRET_ACCESS_KEY=<r2-secret-access-key>
R2_REGION=auto
R2_PUBLIC_BASE_URL=https://<public-r2-domain>
```

Do not commit `.env`. Rotate any provider credentials that were shared outside the secret manager.

## Production Boot

Run migration and seed upsert:

```bash
npm run api:migrate
```

Verify R2:

```bash
npm run api:storage-check
```

Build web:

```bash
npm run build --workspace apps/web
```

Start API:

```bash
npm run api:dev
```

For a real deployment, run the compiled Go server under a process manager or container rather than a dev shell.

## Legal Data Policy

The reference PDFs mention MangaDex or other legal APIs as metadata sources. This implementation uses AniList GraphQL and MangaDex REST for metadata, and R2 for licensed uploads/app-owned media.

- AniList endpoint: `https://graphql.anilist.co`
- MangaDex endpoint: `https://api.mangadex.org`
- Exact backend adapter: `GET /sources/anilist/:id`
- MangaDex backend adapter: `GET /sources/mangadex/:id`
- Search fallback: `GET /sources/anilist/search`
- MangaDex search fallback: `GET /sources/mangadex/search`
- Chapter pages: only admin-uploaded licensed media in R2
- Other R2 objects: publisher/admin cover overrides, user avatars, import bundles, and generated derivatives owned by the app or licensed for upload
- AniList and MangaDex covers: referenced by URL as third-party metadata; do not mirror them into R2
- No third-party chapter image scraping

## Database Operations

The API runs migrations on boot and `npm run api:migrate` runs the same schema path as a one-shot operation.

Tables:

- `users`
- `manga`
- `chapters`
- `user_progress`

Neon is selected automatically when `DATABASE_URL` starts with `postgres://` or `postgresql://`.

## Storage Operations

R2 chapter upload path:

```txt
chapters/<manga_id>/<chapter_id>/<uuid>.<ext>
```

The stored `page_urls` are public URLs derived from `R2_PUBLIC_BASE_URL`.

Allowed page extensions:

- `.jpg`
- `.jpeg`
- `.png`
- `.webp`
- `.gif`

## Production Checks

```bash
npm run api:test
npm run lint --workspace apps/web
npm run build --workspace apps/web
npm run api:migrate
npm run api:storage-check
```

## Security Notes

- `JWT_SECRET` protects user sessions.
- Admin mutation routes require a database user with `role=admin` or `ADMIN_SYNC_TOKEN`.
- R2 credentials must have the narrowest bucket permissions possible.
- Neon credentials should be rotated if exposed in chat, logs, screenshots, or commits.
- `source_url` is provenance, not a user-facing outbound link.
