import Link from "next/link";
import { BookOpen, FilePlus2, Upload, Users } from "lucide-react";
import { AdminTokenField } from "@/components/admin/AdminTokenField";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadCatalog } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function AdminPage() {
  const catalog = await loadCatalog();
  const chapterTotal = catalog.reduce((sum, manga) => sum + manga.totalChapters, 0);
  const firstManga = catalog[0]?.slug ?? "one-piece";
  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title="Dashboard" copy="Manage manga, chapters, uploads, and content quality." />
        <AdminTokenField />
        <section className="admin-stats">
          <div><BookOpen size={22} /><strong>{catalog.length}</strong><span>Backend manga</span></div>
          <div><Upload size={22} /><strong>{chapterTotal}</strong><span>Known chapters</span></div>
          <div><Users size={22} /><strong>Auth</strong><span>JWT users table</span></div>
          <div><FilePlus2 size={22} /><strong>API</strong><span>Admin sync route</span></div>
        </section>
        <section className="admin-actions">
          <Link href="/admin/manga">Manga management</Link>
          <Link href="/admin/manga/new">Add new manga</Link>
          <Link href={`/admin/manga/${firstManga}/chapters`}>Manage chapters</Link>
        </section>
      </div>
    </main>
  );
}
