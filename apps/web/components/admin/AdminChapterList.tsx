"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { Edit, GripVertical, Plus, Trash2 } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { CatalogManga, Chapter } from "@/lib/catalogData";
import { AdminTokenField, hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";
import styles from "./AdminCatalogRows.module.css";

export function AdminChapterList({ manga, chapters }: { manga: CatalogManga; chapters: Chapter[] }) {
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return chapters;
    return chapters.filter((chapter) => `chapter ${chapter.number} ${chapter.title} ${chapter.publishStatus}`.toLowerCase().includes(needle));
  }, [chapters, query]);

  return (
    <>
      <AdminTokenField />
      <div className="search-strip">
        <input placeholder="Search chapters" value={query} onChange={(event) => setQuery(event.target.value)} />
        <Link className="button-primary reader-button" href={`/admin/manga/${manga.slug}/chapters/new`}>
          <Plus size={15} /> Add chapter
        </Link>
      </div>
      <section className="admin-table">
        {filtered.map((chapter) => (
          <article className={styles.chapterRow} key={chapter.id}>
            <span className={styles.titleCell}>
              <strong><GripVertical size={15} /> Chapter {chapter.number}: {chapter.title}</strong>
              <span>{manga.title}</span>
            </span>
            <small>
              {chapter.pages} pages · {chapter.publishStatus} · {chapter.updatedAt}
            </small>
            <Link href={`/admin/manga/${manga.slug}/chapters/${chapter.id}`}>
              <Edit size={15} /> Edit
            </Link>
            <ChapterDeleteButton mangaId={manga.slug} chapter={chapter} />
          </article>
        ))}
        {filtered.length === 0 ? <article className="empty-state">No uploaded chapters are available for this manga.</article> : null}
      </section>
    </>
  );
}

function ChapterDeleteButton({ mangaId, chapter }: { mangaId: string; chapter: Chapter }) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function removeChapter() {
    const adminCredentials = readAdminCredentials();
    if (!hasAdminCredentials(adminCredentials)) {
      setMessage("Sign in as an admin or enter the server admin token first.");
      return;
    }
    if (!window.confirm(`Delete chapter ${chapter.number}: ${chapter.title}?`)) return;

    setLoading(true);
    setMessage("");
    try {
      await apiFetch(`/admin/manga/${encodeURIComponent(mangaId)}/chapters/${encodeURIComponent(chapter.id)}`, {
        method: "DELETE",
        ...adminCredentials,
      });
      setMessage("Deleted.");
      router.refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Delete failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="admin-action-cell">
      <button type="button" onClick={removeChapter} aria-busy={loading}>
        <Trash2 size={15} /> {loading ? "Deleting" : "Delete"}
      </button>
      {message ? <small className="admin-inline-message">{message}</small> : null}
    </div>
  );
}
