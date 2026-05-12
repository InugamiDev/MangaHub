import { MangaCard } from "@/components/catalog/MangaCard";
import { PageTitle } from "@/components/catalog/PageTitle";
import { loadCatalog, popularTitles } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function PopularPage() {
  const catalog = popularTitles(await loadCatalog());
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Ranking" title="Catalog Ranking" copy="Titles ordered by chapter count from the MangaHub API catalog." />
        <section className="catalog-grid">
          {catalog.map((manga) => <MangaCard manga={manga} key={manga.slug} />)}
          {catalog.length === 0 ? <div className="empty-state">No backend ranking data is available.</div> : null}
        </section>
      </div>
    </main>
  );
}
