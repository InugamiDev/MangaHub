package protocols

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/models"
)

const tcpWriteTimeout = 5 * time.Second
const tcpPendingLimit = 20

type ProgressSyncServer struct {
	Port        string
	Connections map[string]net.Conn
	Broadcast   chan models.ProgressUpdate

	Addr string
	JWT  *auth.JWTManager

	mu      sync.RWMutex
	clients map[string]map[*tcpClient]struct{}
	pending map[string][]models.ProgressUpdate
}

type tcpClient struct {
	conn     net.Conn
	userID   string
	username string
	writeMu  sync.Mutex
}

type tcpRequest struct {
	Type  string `json:"type"`
	Token string `json:"token"`
}

type tcpEnvelope struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Payload any    `json:"payload,omitempty"`
}

func NewProgressSyncServer(addr string, jwt *auth.JWTManager) *ProgressSyncServer {
	return &ProgressSyncServer{
		Port:        strings.TrimPrefix(addr, ":"),
		Connections: make(map[string]net.Conn),
		Broadcast:   make(chan models.ProgressUpdate, 64),
		Addr:        addr,
		JWT:         jwt,
		clients:     make(map[string]map[*tcpClient]struct{}),
		pending:     make(map[string][]models.ProgressUpdate),
	}
}

func (s *ProgressSyncServer) Start(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	log.Printf("MangaHub TCP progress sync listening on %s", ln.Addr().String())
	go func() {
		<-ctx.Done()
		_ = ln.Close()
		s.closeAll()
	}()
	go s.consumeBroadcasts(ctx)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			log.Printf("tcp accept error: %v", err)
			continue
		}
		go s.handleConn(conn)
	}
}

func (s *ProgressSyncServer) BroadcastProgress(update models.ProgressUpdate) int {
	clients := s.clientsForUser(update.UserID)
	if len(clients) == 0 {
		s.queuePending(update)
		return 0
	}
	delivered := 0
	for _, client := range clients {
		if client.write(update) == nil {
			delivered++
			continue
		}
		s.unregister(client)
	}
	return delivered
}

func (s *ProgressSyncServer) QueuedProgressCount(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.pending[userID])
}

func (s *ProgressSyncServer) handleConn(conn net.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		_ = conn.Close()
		return
	}
	var req tcpRequest
	if err := json.Unmarshal(scanner.Bytes(), &req); err != nil || strings.ToLower(req.Type) != "auth" || req.Token == "" {
		writeTCP(conn, tcpEnvelope{Type: "error", Message: "first message must be auth with token"})
		_ = conn.Close()
		return
	}
	claims, err := s.JWT.Parse(strings.TrimPrefix(req.Token, "Bearer "))
	if err != nil {
		writeTCP(conn, tcpEnvelope{Type: "error", Message: "invalid or expired token"})
		_ = conn.Close()
		return
	}
	_ = conn.SetReadDeadline(time.Time{})

	client := &tcpClient{conn: conn, userID: claims.UserID, username: claims.Username}
	s.register(client)
	_ = client.write(tcpEnvelope{
		Type:    "ready",
		Message: "progress sync connected",
		Payload: map[string]string{"user_id": claims.UserID, "username": claims.Username},
	})
	s.replayPending(client)
	defer s.unregister(client)

	for scanner.Scan() {
		var incoming tcpRequest
		if err := json.Unmarshal(scanner.Bytes(), &incoming); err != nil {
			_ = client.write(tcpEnvelope{Type: "error", Message: "invalid JSON"})
			continue
		}
		switch strings.ToLower(incoming.Type) {
		case "ping":
			_ = client.write(tcpEnvelope{Type: "pong", Payload: map[string]int64{"timestamp": time.Now().Unix()}})
		default:
			_ = client.write(tcpEnvelope{Type: "error", Message: "unsupported TCP message type"})
		}
	}
}

func (s *ProgressSyncServer) queuePending(update models.ProgressUpdate) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updates := append(s.pending[update.UserID], update)
	if len(updates) > tcpPendingLimit {
		updates = updates[len(updates)-tcpPendingLimit:]
	}
	s.pending[update.UserID] = updates
}

func (s *ProgressSyncServer) replayPending(client *tcpClient) {
	s.mu.Lock()
	updates := append([]models.ProgressUpdate(nil), s.pending[client.userID]...)
	delete(s.pending, client.userID)
	s.mu.Unlock()

	for index, update := range updates {
		if err := client.write(update); err != nil {
			for _, remaining := range updates[index:] {
				s.queuePending(remaining)
			}
			s.unregister(client)
			return
		}
	}
}

func (s *ProgressSyncServer) register(client *tcpClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.clients[client.userID] == nil {
		s.clients[client.userID] = make(map[*tcpClient]struct{})
	}
	s.clients[client.userID][client] = struct{}{}
	s.Connections[client.userID] = client.conn
}

func (s *ProgressSyncServer) unregister(client *tcpClient) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if clients := s.clients[client.userID]; clients != nil {
		delete(clients, client)
		if len(clients) == 0 {
			delete(s.clients, client.userID)
			delete(s.Connections, client.userID)
		} else {
			for remaining := range clients {
				s.Connections[client.userID] = remaining.conn
				break
			}
		}
	}
	_ = client.conn.Close()
}

func (s *ProgressSyncServer) clientsForUser(userID string) []*tcpClient {
	s.mu.RLock()
	defer s.mu.RUnlock()
	clients := make([]*tcpClient, 0, len(s.clients[userID]))
	for client := range s.clients[userID] {
		clients = append(clients, client)
	}
	return clients
}

func (s *ProgressSyncServer) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, clients := range s.clients {
		for client := range clients {
			_ = client.conn.Close()
		}
	}
	s.clients = make(map[string]map[*tcpClient]struct{})
	s.Connections = make(map[string]net.Conn)
}

func (s *ProgressSyncServer) consumeBroadcasts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-s.Broadcast:
			s.BroadcastProgress(update)
		}
	}
}

func (c *tcpClient) write(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return writeTCP(c.conn, value)
}

func writeTCP(conn net.Conn, value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(tcpWriteTimeout))
	payload = append(payload, '\n')
	_, err = conn.Write(payload)
	return err
}
