import { notFound } from "next/navigation";
import { ChapterEditForm } from "@/components/admin/ChapterEditForm";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadChapter, loadManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export async function generateStaticParams() {
  return [];
}

export default async function AdminEditChapterPage({ params }: { params: Promise<{ id: string; chapterId: string }> }) {
  const { id, chapterId } = await params;
  const [manga, chapter] = await Promise.all([loadManga(id), loadChapter(id, chapterId)]);
  if (!manga || !chapter) notFound();

  return (
    <main className="admin-surface">
      <div className="shell">
        <PageTitle eyebrow="Admin" title={`Edit Chapter: ${manga.title}`} copy={`Chapter ${chapter.number}: ${chapter.title}`} />
        <ChapterEditForm mangaId={manga.slug} chapter={chapter} />
      </div>
    </main>
  );
}
