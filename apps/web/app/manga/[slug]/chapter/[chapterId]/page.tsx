import Link from "next/link";
import { notFound } from "next/navigation";
import { ChevronLeft, ChevronRight, List, ShieldCheck } from "lucide-react";
import { RealMangaCover } from "@/components/catalog/RealMangaCover";
import { ChapterSelector } from "@/components/reader/ChapterSelector";
import { loadChapter, loadManga, validImageURL } from "@/lib/catalogData";
import { loadSourceManga } from "@/lib/sourceManga";

export const dynamic = "force-dynamic";

export async function generateStaticParams() {
  return [];
}

export default async function ReaderPage({ params }: { params: Promise<{ slug: string; chapterId: string }> }) {
  const { slug, chapterId } = await params;
  const [manga, chapter] = await Promise.all([loadManga(slug), loadChapter(slug, chapterId)]);
  if (!manga || !chapter) notFound();
  const source = await loadSourceManga(manga);
  const sourceCoverUrl = validImageURL(source?.coverUrl) ? source?.coverUrl : "";
  const chapters = manga.chapters.some((item) => item.id === chapter.id) ? manga.chapters : [chapter, ...manga.chapters];
  const index = chapters.findIndex((item) => item.id === chapterId);
  const previous = chapters[index + 1];
  const next = chapters[index - 1];

  return (
    <main className="reader-page reader-page-with-source">
      {sourceCoverUrl ? <img className="reader-cover-bg" src={sourceCoverUrl} alt="" aria-hidden="true" /> : null}
      <nav className="reader-bar">
        <Link href={`/manga/${manga.slug}`}><ChevronLeft size={16} /> {manga.title}</Link>
        <ChapterSelector mangaSlug={manga.slug} chapters={chapters} currentChapterId={chapter.id} />
        <div>
          <Link className={!previous ? "disabled" : ""} href={previous ? `/manga/${manga.slug}/chapter/${previous.id}` : "#"}>Prev</Link>
          <Link className={!next ? "disabled" : ""} href={next ? `/manga/${manga.slug}/chapter/${next.id}` : "#"}>Next</Link>
        </div>
      </nav>

      <section className="reader-stack">
        <header className="reader-title-card">
          <RealMangaCover title={manga.title} color={manga.color} coverUrl={source?.coverUrl} large />
          <div>
            <p className="kicker">{manga.title} · Chapter {chapter.number}</p>
            <h1>{chapter.title}</h1>
            <p>
              Cover, source, chapter metadata, and page images are loaded through the backend.
            </p>
            <div className="reader-source-actions">
              <span><ShieldCheck size={14} /> {source?.rightsStatus ?? "metadata-only"}</span>
              {source?.sourceProvider ? <span>{source.sourceProvider}</span> : null}
            </div>
          </div>
        </header>
        {chapter.pageUrls.length > 0 ? (
          chapter.pageUrls.map((pageUrl, page) => (
            <figure className="reader-page-image" key={`${pageUrl}-${page}`}>
              <img src={pageUrl} alt={`${manga.title} chapter ${chapter.number} page ${page + 1}`} />
              <figcaption>Page {page + 1}</figcaption>
            </figure>
          ))
        ) : (
          <section className="empty-state">
            No licensed page images have been uploaded for this backend chapter. Use the admin chapter upload flow to attach real pages.
          </section>
        )}
      </section>

      <nav className="reader-bottom">
        <Link href={previous ? `/manga/${manga.slug}/chapter/${previous.id}` : `/manga/${manga.slug}`}><ChevronLeft size={16} /> Previous chapter</Link>
        <Link href={`/manga/${manga.slug}`}><List size={16} /> Back to detail</Link>
        <Link href={next ? `/manga/${manga.slug}/chapter/${next.id}` : `/manga/${manga.slug}`}>Next chapter <ChevronRight size={16} /></Link>
      </nav>
    </main>
  );
}
