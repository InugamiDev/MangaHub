package protocols

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"mangahub/services/api/internal/models"
)

type NotificationServer struct {
	Port    string
	Clients []net.UDPAddr

	Addr string

	mu      sync.RWMutex
	conn    *net.UDPConn
	clients map[string]*net.UDPAddr
}

type udpRequest struct {
	Type string `json:"type"`
}

func NewNotificationServer(addr string) *NotificationServer {
	return &NotificationServer{
		Port:    strings.TrimPrefix(addr, ":"),
		Clients: make([]net.UDPAddr, 0),
		Addr:    addr,
		clients: make(map[string]*net.UDPAddr),
	}
}

func (s *NotificationServer) Start(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", s.Addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.conn = conn
	s.mu.Unlock()
	log.Printf("MangaHub UDP notifications listening on %s", conn.LocalAddr().String())

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buffer := make([]byte, 2048)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if errors.Is(err, net.ErrClosed) || ctx.Err() != nil {
				return nil
			}
			log.Printf("udp read error: %v", err)
			continue
		}
		var req udpRequest
		if err := json.Unmarshal(buffer[:n], &req); err != nil {
			s.send(clientAddr, models.Notification{Type: "error", Message: "invalid JSON", Timestamp: time.Now().Unix()})
			continue
		}
		switch strings.ToLower(req.Type) {
		case "register", "subscribe":
			s.register(clientAddr)
			s.send(clientAddr, models.Notification{Type: "registered", Message: "chapter notifications connected", Timestamp: time.Now().Unix()})
		case "unregister", "unsubscribe":
			s.unregister(clientAddr)
			s.send(clientAddr, models.Notification{Type: "unregistered", Message: "chapter notifications disconnected", Timestamp: time.Now().Unix()})
		case "ping":
			s.send(clientAddr, models.Notification{Type: "pong", Message: "ok", Timestamp: time.Now().Unix()})
		default:
			s.send(clientAddr, models.Notification{Type: "error", Message: "unsupported UDP message type", Timestamp: time.Now().Unix()})
		}
	}
}

func (s *NotificationServer) BroadcastNotification(notification models.Notification) int {
	if notification.Timestamp == 0 {
		notification.Timestamp = time.Now().Unix()
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return 0
	}
	clients := s.clientAddrs()
	conn := s.currentConn()
	if conn == nil {
		return 0
	}
	delivered := 0
	for _, addr := range clients {
		if err := writeUDPWithRetry(conn, payload, addr); err == nil {
			delivered++
		}
	}
	return delivered
}

func writeUDPWithRetry(conn *net.UDPConn, payload []byte, addr *net.UDPAddr) error {
	if _, err := conn.WriteToUDP(payload, addr); err != nil {
		log.Printf("udp broadcast send failed to %s: %v; retrying once", addr.String(), err)
		if _, retryErr := conn.WriteToUDP(payload, addr); retryErr != nil {
			log.Printf("udp broadcast retry failed to %s: %v", addr.String(), retryErr)
			return retryErr
		}
	}
	return nil
}

func (s *NotificationServer) register(addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients[addr.String()] = addr
	s.syncClientListLocked()
}

func (s *NotificationServer) unregister(addr *net.UDPAddr) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.clients, addr.String())
	s.syncClientListLocked()
}

func (s *NotificationServer) send(addr *net.UDPAddr, notification models.Notification) {
	conn := s.currentConn()
	if conn == nil {
		return
	}
	payload, err := json.Marshal(notification)
	if err != nil {
		return
	}
	_, _ = conn.WriteToUDP(payload, addr)
}

func (s *NotificationServer) currentConn() *net.UDPConn {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.conn
}

func (s *NotificationServer) clientAddrs() []*net.UDPAddr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	addrs := make([]*net.UDPAddr, 0, len(s.clients))
	for _, addr := range s.clients {
		addrs = append(addrs, addr)
	}
	return addrs
}

func (s *NotificationServer) syncClientListLocked() {
	s.Clients = s.Clients[:0]
	for _, addr := range s.clients {
		s.Clients = append(s.Clients, *addr)
	}
}
