import {
  apiFetch,
  type Manga,
  type MangaCredit,
  type MangaExternalLink,
  type MangaRanking,
  type MangaRecommendation,
  type MangaRelation,
  type MangaTag,
} from "@/lib/api";
import { safeImageURL, validImageURL, type CatalogManga } from "@/lib/catalogData";

export type SourceManga = {
  title: string;
  author: string;
  coverUrl: string;
  sourceProvider: string;
  sourceUrl: string;
  rightsStatus: string;
  description: string;
  bannerUrl: string;
  startDate: string;
  endDate: string;
  format: string;
  volumes: number;
  meanScore: number;
  averageScore: number;
  popularity: number;
  favourites: number;
  countryOfOrigin: string;
  isLicensed: boolean;
  synonyms: string[];
  tags: MangaTag[];
  relations: MangaRelation[];
  recommendations: MangaRecommendation[];
  characters: MangaCredit[];
  staff: MangaCredit[];
  externalLinks: MangaExternalLink[];
  rankings: MangaRanking[];
};

export async function loadSourceManga(manga: CatalogManga): Promise<SourceManga | null> {
  const backend = await loadBackendManga(manga.slug);
  if (backend && validImageURL(backend.coverUrl) && isLegalMetadataSource(backend)) return backend;

  const sourceUrl = backend?.sourceUrl || manga.sourceUrl;
  const exactMangaDexID = extractMangaDexMangaID(sourceUrl);
  if (exactMangaDexID) {
    const exact = await loadMangaDexSource(exactMangaDexID);
    if (exact) return exact;
  }

  const exactAniListID = extractAniListMangaID(sourceUrl);
  if (exactAniListID) {
    const exact = await loadAniListSource(exactAniListID);
    if (exact) return exact;
  }

  if (backend && validImageURL(backend.coverUrl) && isLegalMetadataSource(backend)) return backend;

  const source = await searchLegalSource(manga.title);
  return source ?? (validImageURL(backend?.coverUrl) ? backend : null);
}

async function loadBackendManga(slug: string): Promise<SourceManga | null> {
  try {
    const manga = await apiFetch<Manga>(`/manga/${encodeURIComponent(slug)}`);
    return mapSourceManga(manga);
  } catch {
    return null;
  }
}

async function searchLegalSource(title: string): Promise<SourceManga | null> {
  const normalized = title.toLowerCase();
  try {
    const payload = await apiFetch<{ results: Manga[] }>(`/sources/anilist/search?q=${encodeURIComponent(title)}&limit=6`);
    const mapped = payload.results.map(mapSourceManga);
    const exact = mapped.find((item) => item.title.toLowerCase() === normalized);
    if (exact) return exact;
  } catch {
    // Fall through to the next legal metadata adapter.
  }
  try {
    const payload = await apiFetch<{ results: Manga[] }>(`/sources/mangadex/search?q=${encodeURIComponent(title)}&limit=6`);
    const mapped = payload.results.map(mapSourceManga);
    return mapped.find((item) => item.title.toLowerCase() === normalized) ?? null;
  } catch {
    return null;
  }
}

async function loadAniListSource(id: string): Promise<SourceManga | null> {
  try {
    const payload = await apiFetch<{ result: Manga }>(`/sources/anilist/${encodeURIComponent(id)}`);
    return mapSourceManga(payload.result);
  } catch {
    return null;
  }
}

async function loadMangaDexSource(id: string): Promise<SourceManga | null> {
  try {
    const payload = await apiFetch<{ result: Manga }>(`/sources/mangadex/${encodeURIComponent(id)}`);
    return mapSourceManga(payload.result);
  } catch {
    return null;
  }
}

function mapSourceManga(manga: Manga): SourceManga {
  return {
    title: manga.title,
    author: manga.author,
    coverUrl: safeImageURL(manga.cover_large_url || manga.cover_url),
    sourceProvider: manga.source_provider || "MangaHub API",
    sourceUrl: manga.source_url || "",
    rightsStatus: manga.rights_status || "metadata-only",
    description: manga.description,
    bannerUrl: safeImageURL(manga.banner_url),
    startDate: manga.start_date || "",
    endDate: manga.end_date || "",
    format: manga.format || "",
    volumes: manga.volumes || 0,
    meanScore: manga.mean_score || 0,
    averageScore: manga.average_score || 0,
    popularity: manga.popularity || 0,
    favourites: manga.favourites || 0,
    countryOfOrigin: manga.country_of_origin || "",
    isLicensed: Boolean(manga.is_licensed),
    synonyms: manga.synonyms ?? [],
    tags: manga.tags ?? [],
    relations: manga.relations ?? [],
    recommendations: manga.recommendations ?? [],
    characters: manga.characters ?? [],
    staff: manga.staff ?? [],
    externalLinks: manga.external_links ?? [],
    rankings: manga.rankings ?? [],
  };
}

function extractAniListMangaID(value?: string) {
  const match = value?.match(/anilist\.co\/manga\/(\d+)/i);
  return match?.[1] ?? "";
}

function extractMangaDexMangaID(value?: string) {
  const match = value?.match(/mangadex\.org\/title\/([0-9a-f-]{36})/i);
  return match?.[1] ?? "";
}

function isLegalMetadataSource(manga: SourceManga) {
  return /(anilist|mangadex)/i.test(`${manga.sourceProvider} ${manga.sourceUrl}`);
}
