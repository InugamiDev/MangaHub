"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { ImagePlus, Upload } from "lucide-react";
import { apiFetch } from "@/lib/api";
import { AdminTokenField, hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";

export function ChapterUploadForm({ mangaId }: { mangaId: string }) {
  const router = useRouter();
  const [title, setTitle] = useState("");
  const [number, setNumber] = useState("");
  const [publishStatus, setPublishStatus] = useState("draft");
  const [files, setFiles] = useState<FileList | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const adminCredentials = readAdminCredentials();
    if (!hasAdminCredentials(adminCredentials)) {
      setMessage("Sign in as an admin or enter the server admin token first.");
      return;
    }
    if (!files?.length) {
      setMessage("Choose at least one page image.");
      return;
    }

    const formData = new FormData();
    formData.set("title", title.trim());
    formData.set("number", number);
    formData.set("publish_status", publishStatus);
    Array.from(files).forEach((file) => formData.append("pages", file));

    setLoading(true);
    setMessage("");
    try {
      await apiFetch(`/admin/manga/${encodeURIComponent(mangaId)}/chapters`, {
        method: "POST",
        ...adminCredentials,
        body: formData,
      });
      router.push(`/admin/manga/${encodeURIComponent(mangaId)}/chapters`);
      router.refresh();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Upload failed.");
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
          <input required placeholder="The Weakest Hunter" value={title} onChange={(event) => setTitle(event.target.value)} />
        </label>
        <label>
          Chapter number
          <input required min="0" step="0.1" type="number" placeholder="1" value={number} onChange={(event) => setNumber(event.target.value)} />
        </label>
        <label>
          Publish status
          <select value={publishStatus} onChange={(event) => setPublishStatus(event.target.value)}>
            <option value="draft">Draft</option>
            <option value="published">Published</option>
            <option value="scheduled">Scheduled</option>
          </select>
        </label>
        <div className="upload-drop">
          <ImagePlus size={32} />
          <strong>Upload manga pages</strong>
          <p>PNG, JPG, or WebP. Files are sent to the backend in selected order.</p>
          <input required name="pages" type="file" multiple accept="image/png,image/jpeg,image/webp" onChange={(event) => setFiles(event.target.files)} />
        </div>
        {message ? <p className="admin-form-message">{message}</p> : null}
        <button type="submit" className="button-primary reader-button" aria-busy={loading}>
          <Upload size={15} /> {loading ? "Uploading" : "Publish chapter"}
        </button>
      </form>
    </>
  );
}
