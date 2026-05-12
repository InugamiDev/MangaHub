"use client";

import Link from "next/link";
import { FormEvent, useEffect, useRef, useState } from "react";
import { MessageCircle, Plug, Send, WifiOff } from "lucide-react";
import { WS_BASE } from "@/lib/api";

type StoredUser = {
  username?: string;
  role?: string;
};

type ChatPayload = {
  status?: string;
  type?: string;
  room?: string;
  user_id?: string;
  username?: string;
  message?: string;
  timestamp?: number;
  error?: string;
};

type ChatLine = {
  id: string;
  kind: "chat" | "system" | "error";
  username?: string;
  message: string;
  timestamp?: number;
};

type ConnectionStatus = "idle" | "connecting" | "connected" | "closed" | "error";

const tokenKey = "mangahub_token";
const userKey = "mangahub_user";

// intent: provide a browser demo for the existing authenticated Go WebSocket chat service
// status: done
// next: extend this with room user counts if the backend exposes presence metadata
// blockers: none
// confidence: high
export function LiveChatPanel() {
  const socketRef = useRef<WebSocket | null>(null);
  const [token, setToken] = useState("");
  const [user, setUser] = useState<StoredUser | null>(null);
  const [room, setRoom] = useState("global");
  const [activeRoom, setActiveRoom] = useState("global");
  const [status, setStatus] = useState<ConnectionStatus>("idle");
  const [message, setMessage] = useState("Hello from the MangaHub website demo.");
  const [lines, setLines] = useState<ChatLine[]>([
    {
      id: createLineID(),
      kind: "system",
      message: "Login, connect, then send a message to prove WebSocket chat is live.",
    },
  ]);

  useEffect(() => {
    const storedToken = window.localStorage.getItem(tokenKey)?.trim() ?? "";
    const storedUser = readStoredUser();
    const line: ChatLine = {
      id: createLineID(),
      kind: "system",
      message: storedToken ? "Loaded website login token." : "No website login token found.",
    };
    setToken(storedToken);
    setUser(storedUser);
    setLines((items) => [...items, line].slice(-40));
    return () => {
      socketRef.current?.close(1000, "page closed");
    };
  }, []);

  function refreshSession() {
    const storedToken = window.localStorage.getItem(tokenKey)?.trim() ?? "";
    const storedUser = readStoredUser();
    setToken(storedToken);
    setUser(storedUser);
    appendLine({ kind: "system", message: storedToken ? "Loaded website login token." : "No website login token found." });
  }

  function appendLine(line: Omit<ChatLine, "id">) {
    setLines((items) => [...items, { ...line, id: createLineID() }].slice(-40));
  }

  function connect() {
    const currentToken = token || window.localStorage.getItem(tokenKey)?.trim() || "";
    if (!currentToken) {
      setStatus("error");
      appendLine({ kind: "error", message: "Login is required before opening WebSocket chat." });
      return;
    }

    socketRef.current?.close(1000, "reconnecting");
    const roomName = room.trim() || "global";
    const url = new URL(`${WS_BASE}/ws/chat`);
    url.searchParams.set("token", currentToken);
    url.searchParams.set("room", roomName);

    setStatus("connecting");
    setActiveRoom(roomName);
    appendLine({ kind: "system", message: `Connecting to ${url.origin}/ws/chat in room ${roomName}.` });

    const socket = new WebSocket(url.toString());
    socketRef.current = socket;

    socket.onopen = () => {
      setStatus("connecting");
    };

    socket.onmessage = (event) => {
      const payload = parsePayload(event.data);
      if (payload.error) {
        setStatus("error");
        appendLine({ kind: "error", message: payload.error });
        return;
      }

      if (payload.status === "connected") {
        setStatus("connected");
        setActiveRoom(payload.room ?? roomName);
        appendLine({ kind: "system", message: `Connected as ${payload.username ?? "reader"} in ${payload.room ?? roomName}.` });
        return;
      }

      if (payload.type === "pong") {
        appendLine({ kind: "system", message: "Pong received from WebSocket service.", timestamp: payload.timestamp });
        return;
      }

      if (payload.message) {
        appendLine({
          kind: "chat",
          username: payload.username ?? "system",
          message: payload.message,
          timestamp: payload.timestamp,
        });
      }
    };

    socket.onerror = () => {
      setStatus("error");
      appendLine({ kind: "error", message: "WebSocket connection error. Confirm the API WebSocket service is listening on port 9093." });
    };

    socket.onclose = (event) => {
      if (socketRef.current === socket) {
        socketRef.current = null;
      }
      setStatus((current) => (current === "error" ? current : "closed"));
      appendLine({ kind: "system", message: `WebSocket closed${event.reason ? `: ${event.reason}` : "."}` });
    };
  }

  function disconnect() {
    socketRef.current?.close(1000, "demo disconnected");
    socketRef.current = null;
  }

  function sendMessage(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const trimmed = message.trim();
    if (!trimmed) return;
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      appendLine({ kind: "error", message: "Connect to WebSocket chat before sending." });
      return;
    }
    socket.send(JSON.stringify({ type: "message", message: trimmed }));
    setMessage("");
  }

  function ping() {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      appendLine({ kind: "error", message: "Connect before sending a ping." });
      return;
    }
    socket.send(JSON.stringify({ type: "ping" }));
  }

  const connected = status === "connected";

  return (
    <section className="live-chat-card" aria-label="WebSocket chat demo">
      <div className="live-chat-toolbar">
        <div>
          <span className={`chat-status-pill ${status}`}>{status}</span>
          <h2>WebSocket Room</h2>
          <p>Authenticated browser chat using the same backend endpoint as the CLI.</p>
        </div>
        <div className="chat-user-badge">
          {user?.username ? `${user.username} - ${user.role ?? "reader"}` : "Signed out"}
        </div>
      </div>

      <div className="chat-control-grid">
        <label className="field">
          <span>Room</span>
          <input value={room} onChange={(event) => setRoom(event.target.value)} placeholder="global or manga:20th-century-boys" />
        </label>
        <div className="chat-actions">
          <button className="button-primary reader-button" type="button" onClick={connect}>
            <Plug size={16} /> Connect
          </button>
          <button className="button-secondary reader-button" type="button" onClick={disconnect} disabled={!socketRef.current}>
            <WifiOff size={16} /> Disconnect
          </button>
          <button className="button-secondary reader-button" type="button" onClick={refreshSession}>
            Refresh session
          </button>
        </div>
      </div>

      {!token ? (
        <div className="chat-login-hint">
          <MessageCircle size={17} />
          <span>
            Website login is required. <Link href="/login">Login here</Link>, then return and refresh the session.
          </span>
        </div>
      ) : null}

      <div className="chat-room-row">
        <span>Endpoint: {WS_BASE}/ws/chat</span>
        <span>Active room: {activeRoom}</span>
      </div>

      <div className="chat-message-list" aria-live="polite">
        {lines.map((line) => (
          <article className={`chat-message ${line.kind}`} key={line.id}>
            <header>
              <strong>{line.kind === "chat" ? line.username : line.kind}</strong>
              <time>{formatTimestamp(line.timestamp)}</time>
            </header>
            <p>{line.message}</p>
          </article>
        ))}
      </div>

      <form className="chat-compose" onSubmit={sendMessage}>
        <input
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          placeholder={connected ? "Type a message for the room" : "Connect before sending"}
          aria-label="Chat message"
        />
        <button className="button-secondary reader-button" type="button" onClick={ping}>
          Ping
        </button>
        <button className="button-primary reader-button" type="submit">
          <Send size={16} /> Send
        </button>
      </form>
    </section>
  );
}

function readStoredUser(): StoredUser | null {
  const raw = window.localStorage.getItem(userKey);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StoredUser;
  } catch {
    return null;
  }
}

function parsePayload(raw: unknown): ChatPayload {
  if (typeof raw !== "string") return {};
  try {
    return JSON.parse(raw) as ChatPayload;
  } catch {
    return { message: raw };
  }
}

function createLineID() {
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function formatTimestamp(timestamp?: number) {
  if (!timestamp) return "now";
  const milliseconds = timestamp < 1_000_000_000_000 ? timestamp * 1000 : timestamp;
  return new Intl.DateTimeFormat(undefined, { hour: "2-digit", minute: "2-digit", second: "2-digit" }).format(milliseconds);
}
