import Link from "next/link";
import { PageTitle } from "@/components/catalog/PageTitle";
import { genresFromCatalog, genreSlug, loadCatalog } from "@/lib/catalogData";

export const dynamic = "force-dynamic";

export default async function GenresPage() {
  const catalog = await loadCatalog();
  const genres = genresFromCatalog(catalog);
  return (
    <main className="page-surface">
      <div className="shell">
        <PageTitle eyebrow="Index" title="Genres" copy="Explore the catalog by mood, format, and story type." />
        <section className="genre-index">
          {genres.map((genre) => {
            const count = catalog.filter((manga) => manga.genres.includes(genre)).length;
            return (
              <Link href={`/genres/${genreSlug(genre)}`} key={genre}>
                <span>{genre}</span>
                <small>{count} titles</small>
              </Link>
            );
          })}
          {genres.length === 0 ? <p className="empty-state">No backend genres are available yet.</p> : null}
        </section>
      </div>
    </main>
  );
}
