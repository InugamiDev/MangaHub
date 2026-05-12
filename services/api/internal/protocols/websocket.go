package protocols

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/models"

	"github.com/gorilla/websocket"
)

const (
	chatWriteTimeout  = 8 * time.Second
	chatReadTimeout   = 75 * time.Second
	maxChatMessageLen = 1000
)

type ChatServer struct {
	Addr          string
	JWT           *auth.JWTManager
	AllowedOrigin string
	hub           *ChatHub
}

type ChatHub struct {
	Clients    map[*websocket.Conn]string
	Broadcast  chan models.ChatMessage
	Register   chan ClientConnection
	Unregister chan *websocket.Conn

	mu             sync.RWMutex
	clients        map[*chatClient]struct{}
	history        map[string][]models.ChatMessage
	connectionRoom map[*websocket.Conn]string
}

type ClientConnection struct {
	Conn     *websocket.Conn
	UserID   string
	Username string
	Room     string
}

type chatClient struct {
	conn     *websocket.Conn
	room     string
	userID   string
	username string
	writeMu  sync.Mutex
}

type wsRequest struct {
	Type    string `json:"type"`
	Token   string `json:"token"`
	Room    string `json:"room"`
	MangaID string `json:"manga_id"`
	Message string `json:"message"`
}

func NewChatServer(addr string, jwt *auth.JWTManager, allowedOrigin string) *ChatServer {
	return &ChatServer{
		Addr:          addr,
		JWT:           jwt,
		AllowedOrigin: allowedOrigin,
		hub: &ChatHub{
			Clients:        make(map[*websocket.Conn]string),
			Broadcast:      make(chan models.ChatMessage, 64),
			Register:       make(chan ClientConnection, 16),
			Unregister:     make(chan *websocket.Conn, 16),
			clients:        make(map[*chatClient]struct{}),
			history:        make(map[string][]models.ChatMessage),
			connectionRoom: make(map[*websocket.Conn]string),
		},
	}
}

func (s *ChatServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"healthy","service":"websocket-chat"}`))
	})
	mux.HandleFunc("/ws/chat", s.handleChat)

	server := &http.Server{Addr: s.Addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("MangaHub WebSocket chat listening on %s", s.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *ChatServer) handleChat(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return s.originAllowed(r.Header.Get("Origin"))
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	req := wsRequest{
		Token: r.URL.Query().Get("token"),
		Room:  r.URL.Query().Get("room"),
	}
	if req.Room == "" && r.URL.Query().Get("manga_id") != "" {
		req.Room = "manga:" + r.URL.Query().Get("manga_id")
	}
	if req.Token == "" {
		_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		if err := conn.ReadJSON(&req); err != nil || strings.ToLower(req.Type) != "join" || req.Token == "" {
			_ = conn.WriteJSON(map[string]string{"error": "first message must be join with token"})
			_ = conn.Close()
			return
		}
		_ = conn.SetReadDeadline(time.Time{})
	}

	claims, err := s.JWT.Parse(strings.TrimPrefix(req.Token, "Bearer "))
	if err != nil {
		_ = conn.WriteJSON(map[string]string{"error": "invalid or expired token"})
		_ = conn.Close()
		return
	}

	room := normalizeRoom(req.Room, req.MangaID)
	client := &chatClient{conn: conn, room: room, userID: claims.UserID, username: claims.Username}
	s.hub.register(client)
	defer func() {
		s.hub.unregister(client)
		s.hub.broadcast(models.ChatMessage{
			UserID:    claims.UserID,
			Username:  claims.Username,
			Message:   claims.Username + " left",
			Timestamp: time.Now().Unix(),
		}, room)
	}()

	client.write(map[string]any{"status": "connected", "room": room, "user_id": claims.UserID, "username": claims.Username})
	for _, message := range s.hub.recent(room) {
		client.write(message)
	}
	s.hub.broadcast(models.ChatMessage{
		UserID:    claims.UserID,
		Username:  claims.Username,
		Message:   claims.Username + " joined",
		Timestamp: time.Now().Unix(),
	}, room)

	s.readLoop(client)
}

func (s *ChatServer) readLoop(client *chatClient) {
	defer client.conn.Close()
	client.conn.SetReadLimit(2048)
	_ = client.conn.SetReadDeadline(time.Now().Add(chatReadTimeout))
	client.conn.SetPongHandler(func(string) error {
		return client.conn.SetReadDeadline(time.Now().Add(chatReadTimeout))
	})
	for {
		var req wsRequest
		if err := client.conn.ReadJSON(&req); err != nil {
			return
		}
		switch strings.ToLower(req.Type) {
		case "ping":
			client.write(map[string]any{"type": "pong", "timestamp": time.Now().Unix()})
		case "message", "chat", "":
			message := strings.TrimSpace(req.Message)
			if message == "" {
				continue
			}
			if len(message) > maxChatMessageLen {
				client.write(map[string]string{"error": "message too long"})
				continue
			}
			s.hub.broadcast(models.ChatMessage{
				UserID:    client.userID,
				Username:  client.username,
				Message:   message,
				Timestamp: time.Now().Unix(),
			}, client.room)
		default:
			client.write(map[string]string{"error": "unsupported WebSocket message type"})
		}
	}
}

func (s *ChatServer) originAllowed(origin string) bool {
	if origin == "" || s.AllowedOrigin == "" || s.AllowedOrigin == "*" {
		return true
	}
	for _, allowed := range strings.Split(s.AllowedOrigin, ",") {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

func (h *ChatHub) register(client *chatClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client] = struct{}{}
	h.Clients[client.conn] = client.username
	h.connectionRoom[client.conn] = client.room
}

func (h *ChatHub) unregister(client *chatClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, client)
	delete(h.Clients, client.conn)
	delete(h.connectionRoom, client.conn)
	_ = client.conn.Close()
}

func (h *ChatHub) broadcast(message models.ChatMessage, room string) {
	h.mu.Lock()
	h.history[room] = append(h.history[room], message)
	if len(h.history[room]) > 30 {
		h.history[room] = h.history[room][len(h.history[room])-30:]
	}
	clients := make([]*chatClient, 0, len(h.clients))
	for client := range h.clients {
		if client.room == room {
			clients = append(clients, client)
		}
	}
	h.mu.Unlock()

	for _, client := range clients {
		if err := client.write(message); err != nil {
			h.unregister(client)
		}
	}
}

func (h *ChatHub) recent(room string) []models.ChatMessage {
	h.mu.RLock()
	defer h.mu.RUnlock()
	messages := h.history[room]
	copyMessages := make([]models.ChatMessage, len(messages))
	copy(copyMessages, messages)
	return copyMessages
}

func (c *chatClient) write(message any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(chatWriteTimeout))
	return c.conn.WriteJSON(message)
}

func normalizeRoom(room, mangaID string) string {
	room = strings.TrimSpace(room)
	mangaID = strings.TrimSpace(mangaID)
	if room == "" && mangaID != "" {
		room = "manga:" + mangaID
	}
	if room == "" {
		room = "global"
	}
	return room
}
