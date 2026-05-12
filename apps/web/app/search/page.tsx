import { FilterSidebar } from "@/components/catalog/FilterSidebar";
import { MangaCard } from "@/components/catalog/MangaCard";
import { PageTitle } from "@/components/catalog/PageTitle";
import { genresFromCatalog, loadCatalog, searchCatalog, type CatalogManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function SearchPage({ searchParams }: { searchParams: Promise<{ q?: string; genre?: string; status?: string }> }) {
  const { q = "", genre = "", status = "" } = await searchParams;
  const [results, catalog] = await Promise.all([searchCatalog(q), loadCatalog()]);
  const genres = genresFromCatalog(catalog.length > 0 ? catalog : results);
  const filtered = filterResults(results, { genre, status });
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Search" title={q ? `Results for "${q}"` : "Search MangaHub"} copy="Find titles, creators, genres, and chapters." />
        <form className="search-strip" action="/search">
          <input name="q" defaultValue={q} placeholder="Search by title, creator, or tag" />
          <input type="hidden" name="genre" value={genre} />
          <input type="hidden" name="status" value={status} />
          <button type="submit">Search</button>
        </form>
        <div className="library-layout">
          <FilterSidebar genres={genres} selectedGenre={genre} selectedStatus={status} hiddenFields={{ q }} />
          {filtered.length > 0 ? (
            <section className="catalog-grid">
              {filtered.map((manga) => <MangaCard manga={manga} key={manga.slug} />)}
            </section>
          ) : (
            <section className="empty-state">No results found. Try a broader title, creator, or genre.</section>
          )}
        </div>
        <div className="pagination"><span>{filtered.length} of {results.length} matching catalog records</span></div>
      </div>
    </main>
  );
}

function filterResults(results: CatalogManga[], filters: { genre: string; status: string }) {
  return results.filter((manga) => {
    const matchesGenre = !filters.genre || manga.genres.includes(filters.genre);
    const matchesStatus = !filters.status || manga.status === filters.status;
    return matchesGenre && matchesStatus;
  });
}
