# Use Cases

## Actors

- Manga Reader: tracks manga reading progress.
- Chat User: participates in real-time discussions.
- System Administrator: manages basic operations and notifications.
- TCP Client: connects to progress sync.
- UDP Client: receives notifications.
- WebSocket Client: browser chat connection.
- External APIs: MangaDx or other legal manga data providers.

## User Management

- UC-001 User Registration: user provides username, email, and password; system validates input, hashes password with bcrypt, stores the user in SQLite, and returns confirmation.
- UC-002 User Authentication: user provides username/email and password; system validates credentials, generates a JWT, and allows protected endpoint access.

## Manga Discovery And Library

- UC-003 Search Manga: user searches by title or author; system applies optional genre/status filters and returns paginated results.
- UC-004 View Manga Details: user opens manga details; system returns metadata and user progress when logged in.
- UC-005 Add Manga to Library: authenticated user selects status and current chapter; system creates or updates the library entry.
- UC-006 Update Reading Progress: user updates chapter; system validates the number, saves progress, and triggers TCP sync in the protocol phase.

## Progress Synchronization

- UC-007 Connect to TCP Sync Server: client opens TCP connection; server accepts it, starts a goroutine, validates the user, and registers the connection.
- UC-008 Broadcast Progress Update: progress update enters broadcast channel; server sends JSON messages to relevant connected clients.

## Notification System

- UC-009 Register for UDP Notifications: UDP client sends registration packet; server stores the client address and confirms registration.
- UC-010 Send Chapter Release Notification: administrator triggers notification; UDP server broadcasts it to registered clients.

## Real-Time Chat

- UC-011 Join Chat: browser upgrades to WebSocket, sends credentials, joins chat, and receives recent history.
- UC-012 Send Chat Message: connected user sends a message; server validates and broadcasts it.
- UC-013 Handle User Disconnection: server detects closure, removes client, broadcasts leave event, and frees resources.

## gRPC Internal Services

- UC-014 Retrieve Manga via gRPC: client calls `GetManga`; service queries database and returns protobuf response.
- UC-015 Search Manga via gRPC: client calls `SearchManga`; service applies criteria and returns results.
- UC-016 Update Progress via gRPC: client calls `UpdateProgress`; service validates, updates DB, triggers TCP broadcast, and returns success.

## Bonus Features

- UC-017 Advanced Manga Search: filter by genres, status, rating, year, and sort criteria.
- UC-018 Submit Manga Review: completed reader writes a review and rating.
- UC-019 View Manga Reviews: readers view community reviews and average rating.
- UC-020 Add Friend: user sends request; target accepts; system creates relationship.
- UC-021 View Friend Activity: user sees recent friend completions, reviews, and ratings.
- UC-022 Generate Reading Statistics: system calculates total chapters, favorite genres, patterns, and trends.
- UC-023 View Personal Statistics: user views dashboards and time period breakdowns.
- UC-024 Cache Popular Manga Data: system stores frequently accessed manga data in Redis.

## Error Handling And Recovery

- UC-025 Handle Database Unavailability: return cached reads when possible, queue writes when feasible, show user-friendly errors, and attempt reconnect.
- UC-026 TCP Server Recovery: restart server, support client reconnect, and queue progress updates during downtime.
- UC-027 WebSocket Connection Recovery: client reconnects, history is preserved, and users are notified of status.

## Performance And Security

- UC-028 Support Concurrent Users: handle 50-100 simultaneous users with stable API, DB, TCP, and WebSocket behavior.
- UC-029 Efficient Data Retrieval: paginate large queries, use indexes, and manage database resources.
- UC-030 Validate JWT Tokens: reject invalid and expired tokens, validate claims, and prevent unauthorized access.
- UC-031 Input Validation: block SQL injection, sanitize XSS attempts, enforce length limits, and reject invalid formats.
