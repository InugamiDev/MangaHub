import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, MessageCircle, Network, ShieldCheck } from "lucide-react";
import { LiveChatPanel } from "@/components/realtime/LiveChatPanel";

export const metadata: Metadata = {
  title: "Live Chat | MangaHub",
  description: "Authenticated WebSocket chat demo for MangaHub rooms.",
};

export default function ChatPage() {
  return (
    <main className="app-wrap live-chat-page">
      <div className="shell">
        <div className="section-head chat-page-head">
          <div>
            <span className="eyebrow">
              <MessageCircle size={15} />
              Live protocol demo
            </span>
            <h1 className="section-title">Website WebSocket Chat</h1>
          </div>
          <p className="section-copy">
            Use this page during demos to show the browser joining the Go WebSocket service on port 9093, receiving recent room history, and broadcasting messages in real time.
          </p>
        </div>

        <div className="live-chat-grid">
          <LiveChatPanel />

          <aside className="chat-demo-card">
            <span className="eyebrow">
              <Network size={14} />
              Demo checklist
            </span>
            <h2>How to prove it live</h2>
            <div className="chat-demo-steps">
              <span><ShieldCheck size={16} /> Login through the website so a JWT is stored locally.</span>
              <span><MessageCircle size={16} /> Connect this page to `global` or `manga:20th-century-boys`.</span>
              <span><Network size={16} /> Send the same room message from the CLI to show cross-client fanout.</span>
            </div>
            <Link className="landing-card-link" href="/app">
              Open API console <ArrowRight size={15} />
            </Link>
          </aside>
        </div>
      </div>
    </main>
  );
}
