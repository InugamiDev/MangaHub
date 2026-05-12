"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Save } from "lucide-react";
import { apiFetch, type Manga } from "@/lib/api";
import { AdminTokenField, hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";

type MangaFormState = {
  id: string;
  title: string;
  author: string;
  genres: string;
  status: string;
  total_chapters: string;
  cover_url: string;
  source_provider: string;
  source_url: string;
  rights_status: string;
  description: string;
};

const initialState: MangaFormState = {
  id: "",
  title: "",
  author: "",
  genres: "",
  status: "Ongoing",
  total_chapters: "0",
  cover_url: "",
  source_provider: "MangaHub API",
  source_url: "",
  rights_status: "licensed-upload",
  description: "",
};

export function MangaCreateForm() {
  const router = useRouter();
  const [form, setForm] = useState(initialState);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  function update<K extends keyof MangaFormState>(key: K, value: MangaFormState[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

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
      const payload = {
        ...form,
        id: form.id.trim(),
        title: form.title.trim(),
        author: form.author.trim(),
        genres: form.genres
          .split(",")
          .map((genre) => genre.trim())
          .filter(Boolean),
        total_chapters: Number(form.total_chapters) || 0,
        cover_url: form.cover_url.trim(),
        source_provider: form.source_provider.trim(),
        source_url: form.source_url.trim(),
        rights_status: form.rights_status.trim(),
        description: form.description.trim(),
      };
      const created = await apiFetch<Manga>("/admin/manga", {
        method: "POST",
        ...adminCredentials,
        body: JSON.stringify(payload),
      });
      const mangaId = created.id || payload.id;
      router.push(`/admin/manga/${encodeURIComponent(mangaId)}/chapters`);
      router.refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Create failed.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <AdminTokenField />
      <form className="admin-form" onSubmit={submit}>
        <label>
          Title
          <input required placeholder="Solo Leveling" value={form.title} onChange={(event) => update("title", event.target.value)} />
        </label>
        <label>
          Slug
          <input required placeholder="solo-leveling" value={form.id} onChange={(event) => update("id", event.target.value)} />
        </label>
        <label>
          Cover URL
          <input placeholder="https://example.com/cover.jpg" value={form.cover_url} onChange={(event) => update("cover_url", event.target.value)} />
        </label>
        <label>
          Author
          <input required placeholder="Author name" value={form.author} onChange={(event) => update("author", event.target.value)} />
        </label>
        <label>
          Genres
          <input placeholder="Action, Fantasy, Manhwa" value={form.genres} onChange={(event) => update("genres", event.target.value)} />
        </label>
        <label>
          Status
          <select value={form.status} onChange={(event) => update("status", event.target.value)}>
            <option>Ongoing</option>
            <option>Completed</option>
            <option>Hiatus</option>
          </select>
        </label>
        <label>
          Total chapters
          <input min="0" type="number" value={form.total_chapters} onChange={(event) => update("total_chapters", event.target.value)} />
        </label>
        <label>
          Source provider
          <input value={form.source_provider} onChange={(event) => update("source_provider", event.target.value)} />
        </label>
        <label>
          Source URL
          <input placeholder="https://example.com/title" value={form.source_url} onChange={(event) => update("source_url", event.target.value)} />
        </label>
        <label>
          Rights status
          <input value={form.rights_status} onChange={(event) => update("rights_status", event.target.value)} />
        </label>
        <label className="wide">
          Description
          <textarea required placeholder="Synopsis" value={form.description} onChange={(event) => update("description", event.target.value)} />
        </label>
        {message ? <p className="admin-form-message">{message}</p> : null}
        <button type="submit" className="button-primary reader-button" aria-busy={loading}>
          <Save size={15} /> {loading ? "Saving" : "Save manga"}
        </button>
      </form>
    </>
  );
}
