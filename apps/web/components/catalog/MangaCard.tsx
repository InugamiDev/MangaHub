import Link from "next/link";
import { BookOpen, ShieldCheck } from "lucide-react";
import type { CatalogManga } from "@/lib/catalogData";
import { RealMangaCover } from "./RealMangaCover";

export function MangaCard({ manga, compact = false }: { manga: CatalogManga; compact?: boolean }) {
  return (
    <article className={compact ? "catalog-card compact" : "catalog-card"}>
      <Link href={`/manga/${manga.slug}`} aria-label={manga.title}>
        <RealMangaCover title={manga.title} color={manga.color} coverUrl={manga.coverUrl} decorative />
      </Link>
      <div className="catalog-card-body">
        <Link href={`/manga/${manga.slug}`}>
          <h3>{manga.title}</h3>
        </Link>
        <p>{manga.sourceProvider} · {manga.totalChapters || "?"} chapters</p>
        <div className="mini-meta">
          <span><BookOpen size={12} /> {manga.status}</span>
          <span><ShieldCheck size={12} /> {manga.rightsStatus}</span>
        </div>
      </div>
    </article>
  );
}
