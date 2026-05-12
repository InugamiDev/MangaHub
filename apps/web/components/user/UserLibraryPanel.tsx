"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";
import { Bookmark, Clock3, Trash2 } from "lucide-react";
import { MangaCard } from "@/components/catalog/MangaCard";
import { apiFetch, type LibraryEntry } from "@/lib/api";
import { libraryEntryToManga } from "@/lib/catalogData";

export function UserLibraryPanel({ mode }: { mode: "bookmarks" | "history" }) {
  const [entries, setEntries] = useState<LibraryEntry[]>([]);
  const [message, setMessage] = useState("Loading your backend library...");
  const [busyID, setBusyID] = useState("");

  useEffect(() => {
    const token = localStorage.getItem("mangahub_token");
    if (!token) {
      setMessage("Login required. This page reads /users/library from the backend.");
      return;
    }
    apiFetch<{ entries: LibraryEntry[] }>("/users/library", { token })
      .then((payload) => {
        setEntries(payload.entries);
        setMessage(payload.entries.length ? "" : "Your backend library is empty.");
      })
      .catch((error) => setMessage(error instanceof Error ? error.message : "Failed to load backend library"));
  }, []);

  const manga = useMemo(() => entries.map((entry, index) => libraryEntryToManga(entry, index)), [entries]);

  async function removeEntry(mangaID: string) {
    const token = localStorage.getItem("mangahub_token");
    if (!token) {
      setMessage("Login required. This page reads /users/library from the backend.");
      return;
    }
    setBusyID(mangaID);
    try {
      await apiFetch(`/users/library/${encodeURIComponent(mangaID)}`, { method: "DELETE", token });
      setEntries((current) => current.filter((entry) => entry.manga_id !== mangaID));
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Failed to remove bookmark");
    } finally {
      setBusyID("");
    }
  }

  if (message) {
    return (
      <section className="empty-state">
        <p>{message}</p>
        <Link href="/login">Login</Link>
      </section>
    );
  }

  if (mode === "history") {
    return (
      <section className="update-list">
        {entries.map((entry) => (
          <article key={entry.manga_id}>
            <div className="rank-cover">{entry.title.slice(0, 1)}</div>
            <div>
              <h3>{entry.title}</h3>
              <p>Last chapter: {entry.current_chapter}</p>
              <small><Clock3 size={13} /> {new Date(entry.updated_at).toLocaleString()}</small>
            </div>
            <Link href={`/manga/${entry.manga_id}/chapter/${entry.current_chapter || 1}`}>Continue</Link>
          </article>
        ))}
      </section>
    );
  }

  return (
    <section className="bookmark-list">
      {manga.map((item, index) => {
        const entry = entries[index];
        const continueHref = entry.current_chapter > 0
          ? `/manga/${item.slug}/chapter/${entry.current_chapter}`
          : `/manga/${item.slug}`;
        return (
          <article key={item.slug}>
            <MangaCard manga={item} compact />
            <div>
              <p>Last read: Chapter {entry.current_chapter || "Not started"}</p>
              <Link href={continueHref}><Bookmark size={14} /> Continue</Link>
              <button type="button" onClick={() => removeEntry(entry.manga_id)} aria-busy={busyID === entry.manga_id}>
                <Trash2 size={14} /> {busyID === entry.manga_id ? "Removing" : "Remove"}
              </button>
            </div>
          </article>
        );
      })}
    </section>
  );
}
