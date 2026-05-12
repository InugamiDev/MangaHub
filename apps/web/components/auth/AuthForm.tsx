"use client";

import Link from "next/link";
import { useState } from "react";
import { LockKeyhole, Mail, User } from "lucide-react";
import { apiFetch } from "@/lib/api";

type AuthMode = "login" | "register";

export function AuthForm({ mode }: { mode: AuthMode }) {
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  async function submit() {
    setMessage("");
    if (mode === "register" && password !== confirmPassword) {
      setMessage("Passwords do not match.");
      return;
    }
    setLoading(true);
    try {
      if (mode === "register") {
        await apiFetch("/auth/register", {
          method: "POST",
          body: JSON.stringify({ username, email, password }),
        });
      }
      const payload = await apiFetch<{ token: string; user: { id: string; username: string; email: string; role: string }; expires_at: string }>("/auth/login", {
        method: "POST",
        body: JSON.stringify(mode === "login" ? { username, email, password } : { username, password }),
      });
      localStorage.setItem("mangahub_token", payload.token);
      localStorage.setItem("mangahub_user", JSON.stringify(payload.user));
      setMessage(`Signed in as ${payload.user.username}.`);
      window.location.assign("/profile");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Authentication failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <section className="auth-card">
      <p className="kicker">{mode === "login" ? "Welcome back" : "Join MangaHub"}</p>
      <h1>{mode === "login" ? "Login" : "Register"}</h1>
      <label>
        <User size={15} /> Username
        <input value={username} onChange={(event) => setUsername(event.target.value)} placeholder="manga_reader" autoComplete="username" />
      </label>
      <label>
        <Mail size={15} /> Email
        <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" placeholder="reader@mangahub.app" autoComplete="email" />
      </label>
      <label>
        <LockKeyhole size={15} /> Password
        <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" placeholder="Password" autoComplete={mode === "login" ? "current-password" : "new-password"} />
      </label>
      {mode === "register" ? (
        <label>
          <LockKeyhole size={15} /> Confirm password
          <input value={confirmPassword} onChange={(event) => setConfirmPassword(event.target.value)} type="password" placeholder="Confirm password" autoComplete="new-password" />
        </label>
      ) : null}
      <button type="button" className="button-primary reader-button" onClick={submit} disabled={loading}>
        {loading ? "Working..." : mode === "login" ? "Login" : "Create account"}
      </button>
      {message ? <p className="auth-message">{message}</p> : null}
      {mode === "login" ? <Link href="/register">Create an account</Link> : <Link href="/login">Already have an account?</Link>}
      {mode === "login" ? <Link href="/recover">Forgot password?</Link> : null}
      <Link href="/app">Open API console</Link>
    </section>
  );
}
