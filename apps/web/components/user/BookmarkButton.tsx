"use client";

import Link from "next/link";
import { useState } from "react";
import { Bookmark } from "lucide-react";
import { apiFetch } from "@/lib/api";

export function BookmarkButton({ mangaId, title, chapterNumber = 1 }: { mangaId: string; title: string; chapterNumber?: number }) {
  const [message, setMessage] = useState("");
  const [loading, setLoading] = useState(false);

  async function saveBookmark() {
    const token = localStorage.getItem("mangahub_token");
    if (!token) {
      setMessage("Login required to save bookmarks.");
      return;
    }
    setLoading(true);
    setMessage("");
    try {
      await apiFetch("/users/library", {
        method: "POST",
        token,
        body: JSON.stringify({ manga_id: mangaId, status: "reading", current_chapter: chapterNumber }),
      });
      setMessage(`${title} saved to your bookmarks.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Bookmark failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <span className="bookmark-action">
      <button className="button-secondary reader-button" type="button" onClick={saveBookmark} disabled={loading}>
        <Bookmark size={16} /> {loading ? "Saving" : "Bookmark"}
      </button>
      {message ? (
        message.startsWith("Login") ? <Link href="/login">{message}</Link> : <small>{message}</small>
      ) : null}
    </span>
  );
}
