import { notFound } from "next/navigation";
import { MangaCard } from "@/components/catalog/MangaCard";
import { PageTitle } from "@/components/catalog/PageTitle";
import { filterByGenre, genreSlug, genresFromCatalog, loadCatalog, type CatalogManga } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export async function generateStaticParams() {
  const catalog = await loadCatalog();
  return genresFromCatalog(catalog).map((genre) => ({ genre: genreSlug(genre) }));
}

export default async function GenrePage({
  params,
  searchParams,
}: {
  params: Promise<{ genre: string }>;
  searchParams: Promise<{ sort?: string }>;
}) {
  const { genre } = await params;
  const { sort = "popular" } = await searchParams;
  const catalog = await loadCatalog();
  const { label, items } = filterByGenre(catalog, genre);
  if (!label) notFound();
  const sorted = sortGenreItems(items, sort);
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Genre" title={label} copy={`Browse ${items.length} titles under ${label}.`} />
        <form className="search-strip">
          <select name="sort" defaultValue={sort}>
            <option value="popular">Popular</option>
            <option value="latest">Latest</option>
            <option value="az">A-Z</option>
          </select>
          <button type="submit">Apply</button>
        </form>
        <section className="catalog-grid">
          {sorted.map((manga) => <MangaCard manga={manga} key={manga.slug} />)}
        </section>
      </div>
    </main>
  );
}

function sortGenreItems(items: CatalogManga[], sort: string) {
  return [...items].sort((a, b) => {
    if (sort === "az") return a.title.localeCompare(b.title);
    return b.totalChapters - a.totalChapters || a.title.localeCompare(b.title);
  });
}
