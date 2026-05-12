# Protocol Services

## HTTP REST API

Status: implemented.

Responsibilities:

- User registration and login.
- JWT authentication middleware.
- Manga search and detail retrieval.
- User library management.
- Reading progress updates.
- JSON request and response handling.
- Basic CORS and error handling.

## TCP Progress Sync

Status: implemented.

Purpose: keep reading progress synchronized across multiple connected devices.

Server shape:

```go
type ProgressSyncServer struct {
  Port        string
  Connections map[string]net.Conn
  Broadcast   chan ProgressUpdate
}

type ProgressUpdate struct {
  UserID    string `json:"user_id"`
  MangaID   string `json:"manga_id"`
  Chapter   int    `json:"chapter"`
  Timestamp int64  `json:"timestamp"`
}
```

Implemented behavior:

- Accept multiple TCP connections.
- Start a goroutine per connection.
- Authenticate the connected client with the same JWT used by HTTP.
- Broadcast progress updates as JSON.
- Remove disconnected clients.
- Continue broadcasting to healthy clients if one send fails.
- Queue up to 20 updates per user when no client is connected and replay them after the next authenticated connection.

## UDP Notifications

Status: implemented.

Purpose: send lightweight chapter release notifications.

Message shape:

```go
type Notification struct {
  Type      string `json:"type"`
  MangaID   string `json:"manga_id"`
  Message   string `json:"message"`
  Timestamp int64  `json:"timestamp"`
}
```

Implemented behavior:

- Listen on UDP port `9091`.
- Accept registration packets from clients.
- Maintain a client address list.
- Broadcast release notifications to registered clients through `POST /admin/notify`.
- Retry each failed client send once, then log unreachable clients without stopping the broadcast.

## gRPC Internal Service

Status: implemented.

Service shape:

```proto
service MangaService {
  rpc GetManga(GetMangaRequest) returns (MangaResponse);
  rpc SearchManga(SearchRequest) returns (SearchResponse);
  rpc UpdateProgress(ProgressRequest) returns (ProgressResponse);
}
```

Implemented behavior:

- Define Protocol Buffer messages for manga, search, and progress.
- Serve unary RPC calls for manga lookup, search, and progress update.
- Query Neon Postgres or SQLite through the shared database layer.
- Trigger TCP progress broadcast from `UpdateProgress`.
- Use `services/api/proto/mangahub.proto` as the public schema with generated protobuf Go files in `services/api/proto/*.pb.go`.

## WebSocket Chat

Status: implemented.

Purpose: support real-time manga discussions.

Hub shape:

```go
type ChatHub struct {
  Clients    map[*websocket.Conn]string
  Broadcast  chan ChatMessage
  Register   chan ClientConnection
  Unregister chan *websocket.Conn
}

type ChatMessage struct {
  UserID    string `json:"user_id"`
  Username  string `json:"username"`
  Message   string `json:"message"`
  Timestamp int64  `json:"timestamp"`
}
```

Implemented behavior:

- Upgrade HTTP connections to WebSocket.
- Validate user identity.
- Broadcast join and leave events.
- Broadcast chat messages to connected users.
- Enforce message length limits.
- Preserve the last 30 chat messages per room for new connections.

## Integration Rule

The HTTP API remains the main user-facing boundary. Protocol services should integrate through shared models and explicit channels rather than duplicating business logic.
