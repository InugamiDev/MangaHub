import Link from "next/link";
import { notFound } from "next/navigation";
import { BarChart3, BookOpen, CalendarDays, ExternalLink, Heart, Play, ShieldCheck, Sparkles, Star, Users } from "lucide-react";
import { MangaCard } from "@/components/catalog/MangaCard";
import { RealMangaCover } from "@/components/catalog/RealMangaCover";
import { BookmarkButton } from "@/components/user/BookmarkButton";
import { genreSlug, loadCatalog, loadManga, relatedManga, validImageURL } from "@/lib/catalogData";
import { loadSourceManga } from "@/lib/sourceManga";

export const dynamic = "force-dynamic";

export async function generateStaticParams() {
  const catalog = await loadCatalog();
  return catalog.map((manga) => ({ slug: manga.slug }));
}

export default async function MangaDetailPage({ params }: { params: Promise<{ slug: string }> }) {
  const { slug } = await params;
  const [manga, catalog] = await Promise.all([loadManga(slug), loadCatalog()]);
  if (!manga) notFound();
  const source = await loadSourceManga(manga);
  const sourceCoverUrl = validImageURL(source?.coverUrl) ? source?.coverUrl : "";
  const displayCoverUrl = sourceCoverUrl || manga.coverUrl;
  const bannerUrl = validImageURL(source?.bannerUrl) ? source?.bannerUrl : displayCoverUrl;
  const related = relatedManga(manga, catalog);
  const readableChapters = manga.chapters.filter((chapter) => chapter.pages > 0 && chapter.publishStatus !== "source-preview");
  const firstChapter = readableChapters.at(-1) ?? readableChapters[0];
  const score = source?.averageScore || source?.meanScore || manga.averageScore || manga.meanScore;
  const popularity = source?.popularity || manga.popularity;
  const favourites = source?.favourites || manga.favourites;
  const rankings = source?.rankings.length ? source.rankings : manga.rankings;
  const tags = source?.tags.length ? source.tags : manga.tags;
  const recommendations = source?.recommendations.length ? source.recommendations : manga.recommendations;
  const relations = source?.relations.length ? source.relations : manga.relations;
  const staff = source?.staff.length ? source.staff : manga.staff;
  const characters = source?.characters.length ? source.characters : manga.characters;
  const externalLinks = source?.externalLinks.length ? source.externalLinks : manga.externalLinks;

  return (
    <main className="page-surface detail-surface">
      {bannerUrl ? <img className="detail-backdrop-image" src={bannerUrl} alt="" aria-hidden="true" /> : null}
      <div className="shell">
        <section className="detail-hero">
          <RealMangaCover title={manga.title} color={manga.color} coverUrl={displayCoverUrl} large />
          <div>
            <p className="kicker">{source?.format || manga.format || "manga"} · {manga.status}</p>
            <h1>{manga.title}</h1>
            <p className="detail-copy">{source?.description || manga.description}</p>
            <div className="meta-row">
              <span><BookOpen size={15} /> {manga.status}</span>
              <span>{manga.totalChapters || "Unknown"} chapters</span>
              {source?.volumes || manga.volumes ? <span>{source?.volumes || manga.volumes} volumes</span> : null}
              {score ? <span><Star size={15} /> {score}% AniList score</span> : null}
              {popularity ? <span><BarChart3 size={15} /> {popularity.toLocaleString()} popularity</span> : null}
              {source ? <span><ShieldCheck size={15} /> {source.rightsStatus}</span> : null}
            </div>
            <div className="tag-row">
              {manga.genres.map((genre) => (
                <Link href={`/genres/${genreSlug(genre)}`} key={genre}>{genre}</Link>
              ))}
            </div>
            <dl className="detail-facts">
              <div><dt>Author</dt><dd>{manga.author}</dd></div>
              <div><dt>Artist</dt><dd>{manga.artist}</dd></div>
              <div><dt>Chapters</dt><dd>{manga.totalChapters || "Unknown"}</dd></div>
              {source?.startDate || manga.startDate ? <div><dt>Started</dt><dd>{source?.startDate || manga.startDate}</dd></div> : null}
              {source?.endDate || manga.endDate ? <div><dt>Ended</dt><dd>{source?.endDate || manga.endDate}</dd></div> : null}
              {favourites ? <div><dt>Favourites</dt><dd>{favourites.toLocaleString()}</dd></div> : null}
            </dl>
            <div className="hero-actions">
              {firstChapter ? (
                <Link className="button-primary reader-button" href={`/manga/${manga.slug}/chapter/${firstChapter.id}`}>
                  <Play size={16} /> Start Reading
                </Link>
              ) : (
                <span className="button-secondary reader-button" aria-disabled="true">
                  <BookOpen size={16} /> No uploaded pages
                </span>
              )}
              <BookmarkButton mangaId={manga.slug} title={manga.title} chapterNumber={firstChapter?.number ?? 1} />
              {source ? <span className="button-secondary reader-button"><ShieldCheck size={16} /> {source.sourceProvider}</span> : null}
            </div>
          </div>
        </section>

        <section className="content-panel source-panel">
          <ShieldCheck size={20} />
          <div>
            <h2>AniList GraphQL enrichment</h2>
            <p>
              MangaHub loads public manga metadata through the backend from {source?.sourceProvider ?? "the catalog source"}. Readers stay on MangaHub; chapter image files are only served from licensed uploads.
            </p>
            {externalLinks.length > 0 ? (
              <div className="external-link-row">
                {externalLinks.slice(0, 6).map((link) => (
                  <a href={link.url} target="_blank" rel="noreferrer" key={`${link.site}-${link.id}`}>
                    {link.site} <ExternalLink size={13} />
                  </a>
                ))}
              </div>
            ) : null}
          </div>
        </section>

        <section className="detail-stat-grid">
          <article>
            <Star size={18} />
            <strong>{score ? `${score}%` : "Tracked"}</strong>
            <span>AniList score</span>
          </article>
          <article>
            <BarChart3 size={18} />
            <strong>{popularity ? popularity.toLocaleString() : "Catalog"}</strong>
            <span>Popularity</span>
          </article>
          <article>
            <Heart size={18} />
            <strong>{favourites ? favourites.toLocaleString() : "Saved"}</strong>
            <span>Favourites</span>
          </article>
          <article>
            <CalendarDays size={18} />
            <strong>{source?.startDate || manga.startDate || "Known"}</strong>
            <span>Publication start</span>
          </article>
        </section>

        {rankings.length > 0 ? (
          <section className="content-panel">
            <h2>Rankings</h2>
            <div className="ranking-strip">
              {rankings.map((rank) => (
                <span key={rank.id}>#{rank.rank} {rank.context}</span>
              ))}
            </div>
          </section>
        ) : null}

        {tags.length > 0 ? (
          <section className="content-panel">
            <h2>Story tags</h2>
            <div className="tag-cloud-rich">
              {tags.slice(0, 12).map((tag) => (
                <span key={tag.id}>{tag.name}<small>{tag.rank}%</small></span>
              ))}
            </div>
          </section>
        ) : null}

        {characters.length > 0 || staff.length > 0 ? (
          <section className="content-panel detail-columns">
            <div>
              <h2><Users size={18} /> Characters</h2>
              <div className="credit-grid">
                {characters.slice(0, 6).map((person) => (
                  <article key={person.id}>
                    {validImageURL(person.image_url) ? <img src={person.image_url} alt="" /> : null}
                    <strong>{person.name}</strong>
                    <span>{person.role}</span>
                  </article>
                ))}
              </div>
            </div>
            <div>
              <h2><Sparkles size={18} /> Staff</h2>
              <div className="credit-list">
                {staff.slice(0, 8).map((person) => (
                  <span key={`${person.id}-${person.role}`}><strong>{person.name}</strong>{person.role ? ` · ${person.role}` : ""}</span>
                ))}
              </div>
            </div>
          </section>
        ) : null}

        <section className="content-panel">
          <h2>Chapters</h2>
          <div className="chapter-list">
            {readableChapters.length > 0 ? (
              readableChapters.map((chapter) => (
                <Link href={`/manga/${manga.slug}/chapter/${chapter.id}`} key={chapter.id}>
                  <span>Chapter {chapter.number}: {chapter.title}</span>
                  <small>{chapter.updatedAt} · {chapter.pages} pages</small>
                </Link>
              ))
            ) : (
              <div className="empty-state">No uploaded chapters are available for this manga yet.</div>
            )}
          </div>
        </section>

        <section className="content-panel">
          <h2>Related manga</h2>
          <div className="catalog-grid related">
            {related.map((item) => (
              <MangaCard manga={item} key={item.slug} compact />
            ))}
          </div>
        </section>

        {relations.length > 0 || recommendations.length > 0 ? (
          <section className="content-panel detail-columns">
            <div>
              <h2>Relations</h2>
              <div className="source-title-list">
                {relations.map((item) => (
                  <span key={item.id}>{item.title}<small>{item.relation_type}</small></span>
                ))}
              </div>
            </div>
            <div>
              <h2>AniList recommendations</h2>
              <div className="source-title-list">
                {recommendations.map((item) => (
                  <span key={item.id}>{item.title}<small>{item.rating} rating</small></span>
                ))}
              </div>
            </div>
          </section>
        ) : null}
      </div>
    </main>
  );
}
