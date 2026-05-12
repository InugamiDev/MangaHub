import Link from "next/link";
import { ArrowRight, BadgeCheck, BookOpen, Database, Flame, Library, Play, Search, ShieldCheck, Sparkles, Star } from "lucide-react";
import { CatalogCover } from "@/components/catalog/CatalogCover";
import { apiFetch } from "@/lib/api";
import { genreSlug, genresFromCatalog, loadCatalogWithChapters, validImageURL, type CatalogManga } from "@/lib/catalogData";
import { loadSourceManga } from "@/lib/sourceManga";

export const dynamic = "force-dynamic";

type LegalSource = {
  id: string;
  name: string;
  kind: string;
  rights_status: string;
  endpoint: string;
};

async function loadBackendManga() {
  const items = await loadCatalogWithChapters(240);
  return { source: items.length > 0 ? "backend" : "empty", items };
}

async function loadSources() {
  try {
    const payload = await apiFetch<{ sources: LegalSource[] }>("/sources");
    return payload.sources;
  } catch {
    return [];
  }
}

function mangaHref(manga: CatalogManga) {
  return `/manga/${manga.slug}`;
}

export default async function HomePage() {
  const [{ items: backendManga }, legalSources] = await Promise.all([loadBackendManga(), loadSources()]);
  const baseFeatured = backendManga.find((item) => item.title.toLowerCase().includes("one piece")) ?? backendManga[0];
  const artworkBySlug = await loadArtworkOverrides(uniqueManga([baseFeatured, ...backendManga.slice(0, 32)]));
  const manga = backendManga.map((item) => artworkBySlug.get(item.slug) ?? item);
  const featured = manga.find((item) => item.title.toLowerCase().includes("one piece")) ?? manga[0];
  const coverWall = manga.filter((item) => validImageURL(item.coverUrl)).slice(0, 14);
  const trending = manga.slice(0, 8);
  const latest = imageFirst(manga.slice(4), manga, 8);
  const genrePreview = genresFromCatalog(manga).slice(0, 12);
  const sourceLabel = "MangaHub API catalog";
  const featuredStartChapter = featured?.chapters.at(-1) ?? featured?.chapters[0];
  const featuredReaderHref = featured
    ? featuredStartChapter
      ? `/manga/${featured.slug}/chapter/${featuredStartChapter.id}`
      : mangaHref(featured)
    : "/library";
  const readerItems = manga.filter((item) => item.chapters.length > 0).slice(0, 3);

  return (
    <main className="landing-page">
      <div className="cover-backdrop" aria-hidden="true">
        <div className="cover-backdrop-track">
          {(coverWall.length > 0 ? coverWall : backendManga).slice(0, 14).map((item, index) => (
            <CoverTile manga={item} priority={index < 4} key={`${item.slug}-${index}`} />
          ))}
        </div>
      </div>

      <section className="landing-hero">
        <div className="shell landing-hero-grid">
          <div className="landing-copy">
            <span className="landing-eyebrow">
              <BadgeCheck size={16} />
              {sourceLabel}
            </span>
            <h1>Manga discovery, tracking, and reading built around legal metadata sources.</h1>
            <p>
              MangaHub loads catalog records through the Go API, enriches titles with legal metadata, and keeps reader progress synced.
            </p>
            <div className="landing-actions">
              <Link className="landing-primary" href="/library">
                <Library size={18} /> Browse Library
              </Link>
              <Link className="landing-secondary" href={featuredReaderHref}>
                <Play size={18} /> Try Reader
              </Link>
            </div>
          </div>

          {featured ? (
            <article className="featured-manga-card">
              <div className="featured-cover-wrap">
                <CoverTile manga={featured} large priority />
                <span className="featured-rank">#01</span>
              </div>
              <div className="featured-copy">
                <span className="fresh-pill">
                  <Flame size={15} />
                  Featured
                </span>
                <h2>{featured.title}</h2>
                <p>{featured.description}</p>
                <div className="source-row">
                  <span><ShieldCheck size={14} /> {featured.rightsStatus}</span>
                  <span>{featured.sourceProvider}</span>
                </div>
                <Link className="landing-card-link" href={mangaHref(featured)}>
                  Open title <ArrowRight size={15} />
                </Link>
              </div>
            </article>
          ) : null}
        </div>
      </section>

      <section className="source-band">
        <div className="shell source-band-grid">
          {legalSources.map((item) => (
            <article key={item.id}>
              <Database size={20} />
              <div>
                <h2>{item.name}</h2>
                <p>{item.kind}</p>
                <small>{item.rights_status}</small>
              </div>
            </article>
          ))}
          {legalSources.length === 0 ? (
            <article>
              <Database size={20} />
              <div>
                <h2>No source records returned</h2>
                <p>Source cards are loaded from the MangaHub API.</p>
                <small>Configure backend sources to populate this section.</small>
              </div>
            </article>
          ) : null}
        </div>
      </section>

      <section className="landing-section shell">
        <div className="landing-section-head">
          <div>
            <p className="kicker">Trending</p>
            <h2>What readers are opening now</h2>
          </div>
          <Link href="/popular">View ranking <ArrowRight size={15} /></Link>
        </div>
        <div className="landing-manga-grid">
          {trending.map((item) => (
            <article className="landing-manga-card" key={item.slug}>
              <Link href={mangaHref(item)}>
                <CoverTile manga={item} />
              </Link>
              <h3>{item.title}</h3>
              <p>{item.author}</p>
              <div>
                <span><Star size={12} /> {item.status}</span>
                <span>{item.totalChapters || "?"} ch.</span>
              </div>
            </article>
          ))}
          {trending.length === 0 ? <article className="empty-state">No catalog records are available yet.</article> : null}
        </div>
      </section>

      <section id="updates" className="landing-section landing-split shell">
        <div>
          <div className="landing-section-head compact">
            <div>
              <p className="kicker">Updates</p>
              <h2>Latest backend records</h2>
            </div>
            <Link href="/latest">All updates <ArrowRight size={15} /></Link>
          </div>
          <div className="landing-update-list">
            {latest.map((item) => (
              <Link href={mangaHref(item)} key={item.slug}>
                <span className="landing-update-cover">
                  <CoverTile manga={item} />
                </span>
                <div>
                  <h3>{item.title}</h3>
                  <p>{item.genres.slice(0, 3).join(", ")}</p>
                </div>
                <small>{item.sourceProvider}</small>
              </Link>
            ))}
            {latest.length === 0 ? <div className="empty-state">No recent catalog records are available yet.</div> : null}
          </div>
        </div>

        <aside className="backend-panel">
          <div className="backend-cover-stream" aria-hidden="true">
            {latest.slice(0, 5).map((item) => (
              <CoverTile manga={item} key={`backend-${item.slug}`} />
            ))}
          </div>
          <p className="kicker">Backend bridge</p>
          <h2>Catalog and accounts share one API surface.</h2>
          <div className="backend-steps">
            <span><Search size={16} /> `/manga` search and filters</span>
            <span><BookOpen size={16} /> `/users/library` bookmarks</span>
            <span><Sparkles size={16} /> `/sources/anilist/:id` metadata adapter</span>
          </div>
          <Link href="/app">Open API console <ArrowRight size={15} /></Link>
        </aside>
      </section>

      <section className="landing-section genre-section">
        <div className="shell genre-grid">
          <div>
            <p className="kicker">Genres</p>
            <h2>Fast routes into the catalog</h2>
          </div>
          <div className="landing-genre-cloud">
            {genrePreview.map((genre) => (
              <Link href={`/genres/${genreSlug(genre)}`} key={genre}>
                {genre}
              </Link>
            ))}
          </div>
        </div>
      </section>

      <section className="landing-section shell reader-showcase">
        <div>
          <p className="kicker">Reader</p>
          <h2>Clean chapter pages with progress paths ready for account sync.</h2>
          <p>Vertical pages, chapter navigation, reading settings, and manga detail links are already wired into the route map.</p>
          <Link className="landing-primary" href={featuredReaderHref}>
            Open reader <ArrowRight size={16} />
          </Link>
        </div>
        <div className="landing-update-list">
          {readerItems.map((item) => (
            <Link href={`/manga/${item.slug}/chapter/${item.chapters[0].id}`} key={item.slug}>
              <span className="landing-update-cover">
                <CoverTile manga={item} />
              </span>
              <div>
                <h3>{item.title}</h3>
                <p>Continue from backend chapter {item.chapters[0].number}</p>
              </div>
              <small>{item.rightsStatus}</small>
            </Link>
          ))}
          {readerItems.length === 0 ? (
            <Link href="/admin/manga">
              <span>+</span>
              <div>
                <h3>No uploaded chapters yet</h3>
                <p>Add a chapter in admin to activate reader links.</p>
              </div>
              <small>Backend required</small>
            </Link>
          ) : null}
        </div>
      </section>

      <section className="landing-cta">
        <div className="shell">
          <p className="kicker">Production path</p>
          <h2>Ready for real catalog ingestion, admin uploads, and authenticated reading lists.</h2>
          <div className="landing-actions">
            <Link className="landing-primary" href="/admin/manga">
              Manage Manga
            </Link>
            <Link className="landing-secondary" href="/login">
              Login
            </Link>
          </div>
        </div>
      </section>

      <footer className="site-footer">
        <div className="shell footer-grid">
          <div className="footer-brand">
            <Link href="/" className="brand" aria-label="MangaHub home">
              <span className="brand-mark">M</span>
              <span>MangaHub</span>
            </Link>
            <p>Legal metadata discovery, personal library tracking, and a responsive manga reader surface.</p>
          </div>
          <nav>
            <h2>Browse</h2>
            <Link href="/library">Library</Link>
            <Link href="/latest">Latest</Link>
            <Link href="/popular">Popular</Link>
            <Link href="/genres">Genres</Link>
          </nav>
          <nav>
            <h2>Account</h2>
            <Link href="/bookmarks">Bookmarks</Link>
            <Link href="/history">History</Link>
            <Link href="/profile">Profile</Link>
            <Link href="/login">Login</Link>
          </nav>
          <nav>
            <h2>Sources</h2>
            <span>AniList metadata</span>
            <span>MANGA Plus reference</span>
            <Link href="/admin">Admin</Link>
            <Link href="/app">API Console</Link>
          </nav>
        </div>
        <div className="shell footer-bottom">
          <span>Metadata-only source adapters. Chapter images require publisher authorization.</span>
          <span>Terms / Privacy / Community Guidelines</span>
        </div>
      </footer>
    </main>
  );
}

function uniqueManga(items: Array<CatalogManga | undefined>) {
  const seen = new Set<string>();
  return items.filter((item): item is CatalogManga => {
    if (!item || seen.has(item.slug)) return false;
    seen.add(item.slug);
    return true;
  });
}

async function loadArtworkOverrides(items: CatalogManga[]) {
  const entries = await Promise.all(
    items.map(async (manga) => {
      const source = await loadSourceManga(manga);
      if (!source || !validImageURL(source.coverUrl)) return [manga.slug, manga] as const;
      return [
        manga.slug,
        {
          ...manga,
          coverUrl: source.coverUrl,
          coverLargeUrl: source.coverUrl,
          bannerUrl: validImageURL(source.bannerUrl) ? source.bannerUrl : manga.bannerUrl,
          sourceProvider: source.sourceProvider || manga.sourceProvider,
          sourceUrl: source.sourceUrl || manga.sourceUrl,
          rightsStatus: source.rightsStatus || manga.rightsStatus,
        },
      ] as const;
    }),
  );
  return new Map(entries);
}

function imageFirst(primary: CatalogManga[], fallback: CatalogManga[], limit: number) {
  const seen = new Set<string>();
  const ordered = [
    ...primary.filter((item) => validImageURL(item.coverUrl)),
    ...fallback.filter((item) => validImageURL(item.coverUrl)),
    ...primary,
    ...fallback,
  ];
  return ordered
    .filter((item) => {
      if (seen.has(item.slug)) return false;
      seen.add(item.slug);
      return true;
    })
    .slice(0, limit);
}

function CoverTile({ manga, large = false, priority = false }: { manga: CatalogManga; large?: boolean; priority?: boolean }) {
  if (validImageURL(manga.coverUrl)) {
    return (
      <img
        className={large ? "real-cover large" : "real-cover"}
        src={manga.coverUrl}
        alt={priority ? `${manga.title} cover` : ""}
        loading={priority ? "eager" : "lazy"}
      />
    );
  }
  return <CatalogCover title={manga.title} color={manga.color} large={large} />;
}
