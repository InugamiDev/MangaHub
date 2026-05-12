"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Bookmark, Clock3, LogOut } from "lucide-react";
import { apiFetch, type LibraryEntry } from "@/lib/api";

type StoredUser = {
  username?: string;
  email?: string;
  role?: string;
};

export function ProfileSummary() {
  const [user, setUser] = useState<StoredUser | null>(null);
  const [entries, setEntries] = useState<LibraryEntry[]>([]);

  useEffect(() => {
    const storedUser = localStorage.getItem("mangahub_user");
    const token = localStorage.getItem("mangahub_token");
    if (storedUser) {
      setUser(JSON.parse(storedUser) as StoredUser);
    }
    if (token) {
      apiFetch<{ user: StoredUser }>("/users/me", { token })
        .then((payload) => {
          setUser(payload.user);
          localStorage.setItem("mangahub_user", JSON.stringify(payload.user));
        })
        .catch(() => undefined);
      apiFetch<{ entries: LibraryEntry[] }>("/users/library", { token })
        .then((payload) => setEntries(payload.entries))
        .catch(() => setEntries([]));
    }
  }, []);

  const initials = (user?.username ?? "Reader").slice(0, 2).toUpperCase();

  return (
    <section className="profile-grid">
      <div className="profile-card">
        <div className="profile-avatar">{initials}</div>
        <h2>{user?.username ?? "Not logged in"}</h2>
        <p>{user?.email ?? "Login to load your backend profile"}{user?.role ? ` · ${user.role}` : ""}</p>
      </div>
      <Link href="/bookmarks"><Bookmark size={18} /> {entries.length} bookmarks</Link>
      <Link href="/history"><Clock3 size={18} /> {entries.length} history items</Link>
      <button
        type="button"
        onClick={() => {
          localStorage.removeItem("mangahub_token");
          localStorage.removeItem("mangahub_user");
          setUser(null);
          setEntries([]);
        }}
      >
        <LogOut size={18} /> Logout
      </button>
    </section>
  );
}
