# Implementation Roadmap

## Phase 1: Foundation And HTTP Delivery

Deliverables:

- Repository scaffold.
- Next.js product site.
- Go HTTP API.
- SQLite schema.
- Seed manga data.
- User registration and login.
- JWT middleware.
- Manga search and details.
- User library and progress update.
- Documentation pack.

Status: implemented in this first pass.

## Phase 2: TCP Progress Synchronization

Deliverables:

- TCP server on port `9090`.
- JSON client registration message.
- Goroutine per connection.
- Broadcast channel for progress updates.
- HTTP progress update integration.
- TCP monitor demo client.

Status: implemented.

Acceptance:

- 20-30 clients can connect during testing.
- Progress update is delivered within 1 second.
- Lost connections are removed without crashing the server.

## Phase 3: UDP Notifications

Deliverables:

- UDP server on port `9091`.
- Client registration packet.
- Notification broadcast command.
- Basic unreachable-client logging.

Status: implemented.

Acceptance:

- Registered clients receive chapter release notifications.
- Missing clients do not stop the broadcast.

## Phase 4: WebSocket Chat

Deliverables:

- WebSocket endpoint on port `9093`.
- Join, message, and leave event handling.
- Basic chat history.
- General room first, manga-specific rooms as bonus.

Status: implemented.

Acceptance:

- 10-20 simultaneous users can chat.
- Messages arrive within 100ms in normal local testing.
- Disconnects clean up resources.

## Phase 5: gRPC Internal Service

Deliverables:

- `proto/manga.proto`.
- `GetManga`, `SearchManga`, and `UpdateProgress`.
- gRPC server on port `9092`.
- Simple CLI or test client.
- Optional server-side streaming as bonus.

Status: implemented for unary calls. Server-side streaming remains optional bonus scope.

Acceptance:

- All three unary methods return expected data.
- `UpdateProgress` reuses validation and triggers TCP broadcast.

## Phase 6: Polish, Testing, And Demo

Deliverables:

- Unit tests for auth, search, library, and progress.
- Integration tests for SQLite-backed flows.
- Protocol smoke tests.
- Demo script rehearsal.
- README and setup verification.

Acceptance:

- Fresh clone can run the API and website.
- Demo flow completes without manual database edits.
- All five protocols are demonstrable.

## Recommended Bonus Choices

Fast bonus features:

- Health checks for every service.
- Input sanitization.
- Notification preferences.
- Multiple reading lists.

Medium bonus features:

- Advanced search and filtering.
- Reviews and ratings.
- Reading statistics.
- WebSocket room management.

Advanced bonus features:

- Redis caching.
- Recommendation system.
- Friend system.
- CI/CD pipeline.
- Docker Compose.
