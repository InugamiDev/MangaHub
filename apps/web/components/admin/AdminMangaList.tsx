"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMemo, useState } from "react";
import { Edit, Plus, Trash2 } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { CatalogManga } from "@/lib/catalogData";
import { AdminTokenField, hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";
import { RealMangaCover } from "@/components/catalog/RealMangaCover";
import styles from "./AdminCatalogRows.module.css";

export function AdminMangaList({ catalog }: { catalog: CatalogManga[] }) {
  const [query, setQuery] = useState("");
  const filtered = useMemo(() => {
    const needle = query.trim().toLowerCase();
    if (!needle) return catalog;
    return catalog.filter((manga) => [manga.title, manga.author, manga.status, manga.sourceProvider].join(" ").toLowerCase().includes(needle));
  }, [catalog, query]);

  return (
    <>
      <AdminTokenField />
      <div className="search-strip">
        <input placeholder="Search manga" value={query} onChange={(event) => setQuery(event.target.value)} />
        <Link className="button-primary reader-button" href="/admin/manga/new">
          <Plus size={15} /> Add manga
        </Link>
      </div>
      <section className="admin-table">
        {filtered.map((manga) => (
          <article className={styles.mangaRow} key={manga.slug}>
            <Link className={styles.thumb} href={`/manga/${manga.slug}`} aria-label={`Open ${manga.title}`}>
              <RealMangaCover title={manga.title} color={manga.color} coverUrl={manga.coverUrl} decorative />
            </Link>
            <span className={styles.titleCell}>
              <strong>{manga.title}</strong>
              <span>{manga.author}</span>
            </span>
            <small>
              {manga.status} · {manga.totalChapters || "unknown"} chapters · {manga.sourceProvider}
            </small>
            <Link href={`/admin/manga/${manga.slug}/chapters`}>
              <Edit size={15} /> Chapters
            </Link>
            <MangaDeleteButton mangaId={manga.slug} title={manga.title} />
          </article>
        ))}
        {filtered.length === 0 ? <article className="empty-state">No backend catalog records match this search.</article> : null}
      </section>
    </>
  );
}

function MangaDeleteButton({ mangaId, title }: { mangaId: string; title: string }) {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function removeManga() {
    const adminCredentials = readAdminCredentials();
    if (!hasAdminCredentials(adminCredentials)) {
      setMessage("Sign in as an admin or enter the server admin token first.");
      return;
    }
    if (!window.confirm(`Delete "${title}" and its chapters?`)) return;

    setLoading(true);
    setMessage("");
    try {
      await apiFetch(`/admin/manga/${encodeURIComponent(mangaId)}`, {
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
      <button type="button" onClick={removeManga} aria-busy={loading}>
        <Trash2 size={15} /> {loading ? "Deleting" : "Delete"}
      </button>
      {message ? <small className="admin-inline-message">{message}</small> : null}
    </div>
  );
}
