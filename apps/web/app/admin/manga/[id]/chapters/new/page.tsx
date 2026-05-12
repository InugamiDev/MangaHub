import { notFound } from "next/navigation";
import { ChapterUploadForm } from "@/components/admin/ChapterUploadForm";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function AdminNewChapterPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const manga = await loadManga(id);
  if (!manga) notFound();
  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title={`Upload Chapter: ${manga.title}`} copy="Publish chapter metadata and page images through the admin API." />
        <ChapterUploadForm mangaId={manga.slug} />
      </div>
    </main>
  );
}
