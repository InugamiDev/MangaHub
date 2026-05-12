"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { BarChart3, Bookmark, Clock3, LogOut, Star } from "lucide-react";
import { apiFetch, type LibraryEntry, type UserStats } from "@/lib/api";

type StoredUser = {
  username?: string;
  email?: string;
  role?: string;
};

export function ProfileSummary() {
  const [user, setUser] = useState<StoredUser | null>(null);
  const [entries, setEntries] = useState<LibraryEntry[]>([]);
  const [stats, setStats] = useState<UserStats | null>(null);

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
      apiFetch<{ stats: UserStats }>("/users/stats", { token })
        .then((payload) => setStats(payload.stats))
        .catch(() => setStats(null));
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
      <Link href="/friends"><BarChart3 size={18} /> {stats?.total_chapters_read ?? 0} chapters read</Link>
      <span><Star size={18} /> {stats?.review_count ?? 0} reviews · {(stats?.average_rating ?? 0).toFixed(1)} avg</span>
      <button
        type="button"
        onClick={() => {
          localStorage.removeItem("mangahub_token");
          localStorage.removeItem("mangahub_user");
          setUser(null);
          setEntries([]);
          setStats(null);
        }}
      >
        <LogOut size={18} /> Logout
      </button>
    </section>
  );
}
