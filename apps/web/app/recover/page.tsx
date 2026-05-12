"use client";

import { useState } from "react";
import { KeyRound } from "lucide-react";
import { apiFetch } from "@/lib/api";

export default function RecoverPage() {
  const [identifier, setIdentifier] = useState("");
  const [token, setToken] = useState("");
  const [password, setPassword] = useState("");
  const [message, setMessage] = useState("Request a demo recovery token, then reset your password.");
  const [loading, setLoading] = useState(false);

  async function requestRecovery() {
    setLoading(true);
    try {
      const payload = await apiFetch<{ token?: string; status: string }>("/auth/recovery/request", {
        method: "POST",
        body: JSON.stringify(identifier.includes("@") ? { email: identifier } : { username: identifier }),
      });
      if (payload.token) {
        setToken(payload.token);
        setMessage("Recovery token generated for this local demo.");
      } else {
        setMessage("If the account exists, recovery has started.");
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Recovery request failed.");
    } finally {
      setLoading(false);
    }
  }

  async function resetPassword() {
    setLoading(true);
    try {
      await apiFetch("/auth/recovery/reset", {
        method: "POST",
        body: JSON.stringify({ token, new_password: password }),
      });
      setMessage("Password reset. You can log in with the new password.");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Password reset failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="page-surface">
      <div className="shell auth-page-shell">
        {/* intent: expose the local account recovery flow required by the demo use cases */}
        {/* status: done */}
        {/* next: swap visible demo tokens for email delivery before production use */}
        {/* blockers: none */}
        {/* confidence: high */}
        <section className="auth-card recovery-card">
          <p className="kicker">Account recovery</p>
          <h1><KeyRound size={22} /> Reset password</h1>
          <label>
            Username or email
            <input value={identifier} onChange={(event) => setIdentifier(event.target.value)} placeholder="reader@example.com" />
          </label>
          <button type="button" className="button-primary reader-button" onClick={requestRecovery} disabled={loading}>Request token</button>
          <label>
            Recovery token
            <input value={token} onChange={(event) => setToken(event.target.value)} placeholder="rst_..." />
          </label>
          <label>
            New password
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" placeholder="At least 8 characters" />
          </label>
          <button type="button" className="button-secondary reader-button" onClick={resetPassword} disabled={loading}>Reset password</button>
          {message ? <p className="auth-message">{message}</p> : null}
        </section>
      </div>
    </main>
  );
}
