import { notFound } from "next/navigation";
import { AdminChapterList } from "@/components/admin/AdminChapterList";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function AdminChaptersPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const manga = await loadManga(id);
  if (!manga) notFound();
  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title={`${manga.title} Chapters`} copy="Upload, edit, and delete chapter records from the backend." />
        <AdminChapterList manga={manga} chapters={manga.chapters} />
      </div>
    </main>
  );
}
