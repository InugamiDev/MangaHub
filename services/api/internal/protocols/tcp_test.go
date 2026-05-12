package protocols

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"mangahub/services/api/internal/auth"
	"mangahub/services/api/internal/models"
)

func TestProgressSyncTCPHandshakeAndPing(t *testing.T) {
	// intent: exercise TCP protocol behavior without binding ports or requiring a deployment
	// status: done
	// next: keep this as the fast protocol-facing smoke test for go test ./...
	// blockers: none
	// confidence: high
	jwt := auth.NewJWTManager("test-secret")
	token, _, err := jwt.Generate("user-1", "reader")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	server := NewProgressSyncServer(":0", jwt)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	go server.handleConn(serverConn)

	reader := bufio.NewReader(clientConn)
	if _, err := clientConn.Write([]byte(`{"type":"auth","token":"`)); err != nil {
		t.Fatalf("write auth prefix: %v", err)
	}
	if _, err := clientConn.Write([]byte(token + `"}` + "\n")); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	ready := readTCPEnvelope(t, reader)
	if ready.Type != "ready" || ready.Message != "progress sync connected" {
		t.Fatalf("ready envelope = %#v", ready)
	}

	if _, err := clientConn.Write([]byte(`{"type":"ping"}` + "\n")); err != nil {
		t.Fatalf("write ping: %v", err)
	}
	pong := readTCPEnvelope(t, reader)
	if pong.Type != "pong" {
		t.Fatalf("pong envelope = %#v", pong)
	}
}

func readTCPEnvelope(t *testing.T, reader *bufio.Reader) tcpEnvelope {
	t.Helper()
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read envelope: %v", err)
	}
	var envelope tcpEnvelope
	if err := json.NewDecoder(strings.NewReader(line)).Decode(&envelope); err != nil {
		t.Fatalf("decode envelope %q: %v", line, err)
	}
	return envelope
}

func BenchmarkProgressSyncBroadcastToRegisteredClients(b *testing.B) {
	jwt := auth.NewJWTManager("test-secret")
	server := NewProgressSyncServer(":0", jwt)
	for i := 0; i < 8; i++ {
		clientConn, serverConn := net.Pipe()
		defer clientConn.Close()
		defer serverConn.Close()
		server.register(&tcpClient{conn: serverConn, userID: "user-1", username: "reader"})
		go func(conn net.Conn) {
			scanner := bufio.NewScanner(conn)
			for scanner.Scan() {
			}
		}(clientConn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		server.BroadcastProgress(models.ProgressUpdate{UserID: "user-1", MangaID: "naruto", Chapter: i, Timestamp: time.Now().Unix()})
	}
}
