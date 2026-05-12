import { FilterSidebar } from "@/components/catalog/FilterSidebar";
import { MangaCard } from "@/components/catalog/MangaCard";
import { PageTitle } from "@/components/catalog/PageTitle";
import { genresFromCatalog, loadCatalog, type CatalogManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

type LibrarySearchParams = {
  q?: string;
  sort?: string;
  genre?: string;
  status?: string;
};

export default async function LibraryPage({ searchParams }: { searchParams: Promise<LibrarySearchParams> }) {
  const { q = "", sort = "latest", genre = "", status = "" } = await searchParams;
  const catalog = await loadCatalog();
  const genres = genresFromCatalog(catalog);
  const filtered = sortCatalog(filterCatalog(catalog, { q, genre, status }), sort);
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Browse" title="Manga Library" copy="Search, filter, and sort every available series in the catalog." />
        <form className="search-strip">
          <input name="q" defaultValue={q} placeholder="Search by title, author, or genre" />
          <input type="hidden" name="genre" value={genre} />
          <input type="hidden" name="status" value={status} />
          <select name="sort" defaultValue={sort}>
            <option value="latest">Latest</option>
            <option value="popular">Popular</option>
            <option value="az">A-Z</option>
            <option value="chapters">Most chapters</option>
          </select>
          <button type="submit">Search</button>
        </form>
        <div className="library-layout">
          <FilterSidebar genres={genres} selectedGenre={genre} selectedStatus={status} hiddenFields={{ q, sort }} />
          {filtered.length > 0 ? (
            <section className="catalog-grid">
              {filtered.map((manga) => (
                <MangaCard manga={manga} key={manga.slug} />
              ))}
            </section>
          ) : (
            <section className="empty-state">No catalog records match the current search and filters.</section>
          )}
        </div>
        <div className="pagination"><span>{filtered.length} of {catalog.length} catalog records</span></div>
      </div>
    </main>
  );
}

function filterCatalog(catalog: CatalogManga[], filters: { q: string; genre: string; status: string }) {
  const query = filters.q.trim().toLowerCase();
  return catalog.filter((manga) => {
    const matchesQuery = !query || [manga.title, manga.author, manga.genres.join(" ")].join(" ").toLowerCase().includes(query);
    const matchesGenre = !filters.genre || manga.genres.includes(filters.genre);
    const matchesStatus = !filters.status || manga.status === filters.status;
    return matchesQuery && matchesGenre && matchesStatus;
  });
}

function sortCatalog(catalog: CatalogManga[], sort: string) {
  return [...catalog].sort((a, b) => {
    if (sort === "az") return a.title.localeCompare(b.title);
    if (sort === "popular" || sort === "chapters") return b.totalChapters - a.totalChapters || a.title.localeCompare(b.title);
    return b.totalChapters - a.totalChapters || a.title.localeCompare(b.title);
  });
}
