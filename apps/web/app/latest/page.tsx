import Link from "next/link";
import { Clock3 } from "lucide-react";
import { PageTitle } from "@/components/catalog/PageTitle";
import { latestUpdates, loadCatalogWithChapters } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function LatestPage() {
  const catalog = latestUpdates(await loadCatalogWithChapters());
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Updates" title="Latest Backend Chapters" copy="Chapter metadata from the shared MangaHub API catalog." />
        <section className="update-list">
          {catalog.map((manga) => {
            const chapter = manga.chapters[0];
            return (
              <article key={manga.slug}>
                <div className="rank-cover">{manga.title.slice(0, 1)}</div>
                <div>
                  <h3>{manga.title}</h3>
                  <p>{chapter ? `Latest uploaded chapter: ${chapter.number}` : "No uploaded chapters yet"}</p>
                  <small><Clock3 size={13} /> {chapter?.updatedAt ?? manga.sourceProvider}</small>
                </div>
                <Link href={chapter ? `/manga/${manga.slug}/chapter/${chapter.id}` : `/manga/${manga.slug}`}>
                  {chapter ? "Read" : "Details"}
                </Link>
              </article>
            );
          })}
          {catalog.length === 0 ? <article className="empty-state">No backend update records are available.</article> : null}
        </section>
      </div>
    </main>
  );
}
