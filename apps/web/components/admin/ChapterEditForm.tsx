"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Save } from "lucide-react";
import { apiFetch } from "@/lib/api";
import type { Chapter } from "@/lib/catalogData";
import { AdminTokenField, hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";

export function ChapterEditForm({ mangaId, chapter }: { mangaId: string; chapter: Chapter }) {
  const router = useRouter();
  const [title, setTitle] = useState(chapter.title);
  const [number, setNumber] = useState(String(chapter.number));
  const [publishStatus, setPublishStatus] = useState(chapter.publishStatus || "published");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const adminCredentials = readAdminCredentials();
    if (!hasAdminCredentials(adminCredentials)) {
      setMessage("Sign in as an admin or enter the server admin token first.");
      return;
    }

    setLoading(true);
    setMessage("");
    try {
      await apiFetch(`/admin/manga/${encodeURIComponent(mangaId)}/chapters/${encodeURIComponent(chapter.id)}`, {
        method: "PUT",
        ...adminCredentials,
        body: JSON.stringify({
          title: title.trim(),
          number: Number(number),
          publish_status: publishStatus,
        }),
      });
      router.push(`/admin/manga/${encodeURIComponent(mangaId)}/chapters`);
      router.refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Update failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <AdminTokenField />
      <form className="admin-form" onSubmit={submit}>
        <label>
          Chapter title
          <input required value={title} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <label>
          Chapter number
          <input required min="0" step="0.1" type="number" value={number} onChange={(event) => setNumber(event.target.value)} />
        </label>
        <label>
          Publish status
          <select value={publishStatus} onChange={(event) => setPublishStatus(event.target.value)}>
            <option value="draft">Draft</option>
            <option value="published">Published</option>
            <option value="scheduled">Scheduled</option>
          </select>
        </label>
        <p className="admin-form-note">{chapter.pages} uploaded pages.</p>
        {message ? <p className="admin-form-message">{message}</p> : null}
        <button type="submit" className="button-primary reader-button" aria-busy={loading}>
          <Save size={15} /> {loading ? "Saving" : "Save chapter"}
        </button>
      </form>
    </>
  );
}
