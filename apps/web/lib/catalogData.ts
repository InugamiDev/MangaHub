import {
  API_BASE,
  apiFetch,
  type LibraryEntry,
  type Manga,
  type MangaChapter,
  type MangaCredit,
  type MangaExternalLink,
  type MangaRanking,
  type MangaRecommendation,
  type MangaRelation,
  type MangaTag,
} from "@/lib/api";

export type Chapter = {
  id: string;
  title: string;
  number: number;
  updatedAt: string;
  pages: number;
  pageUrls: string[];
  publishStatus: string;
};

export type CatalogManga = {
  slug: string;
  title: string;
  author: string;
  artist: string;
  genres: string[];
  status: "Ongoing" | "Completed" | "Hiatus" | "Cancelled" | "Not Yet Released";
  language: string;
  year: number | null;
  rating: string;
  views: string;
  description: string;
  color: string;
  coverUrl: string;
  sourceProvider: string;
  sourceUrl: string;
  rightsStatus: string;
  totalChapters: number;
  chapters: Chapter[];
  coverLargeUrl: string;
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

export function genreSlug(genre: string) {
  return genre.toLowerCase().replaceAll(" ", "-");
}

export async function loadCatalog(limit = 300): Promise<CatalogManga[]> {
  try {
    const payload = await apiFetch<{ results: Manga[]; count: number }>(`/manga?limit=${limit}`);
    return payload.results.map(mapManga);
  } catch {
    return [];
  }
}

export async function loadCatalogWithChapters(limit = 300): Promise<CatalogManga[]> {
  const catalog = await loadCatalog(limit);
  return Promise.all(
    catalog.map(async (manga) => ({
      ...manga,
      chapters: await loadChapters(manga.slug),
    })),
  );
}

export async function loadManga(slug: string): Promise<CatalogManga | null> {
  try {
    const manga = await apiFetch<Manga>(`/manga/${encodeURIComponent(slug)}`);
    const mapped = mapManga(manga, 0);
    return {
      ...mapped,
      chapters: await loadChapters(slug),
    };
  } catch {
    return null;
  }
}

export async function loadChapters(mangaId: string): Promise<Chapter[]> {
  try {
    const payload = await apiFetch<ChapterListPayload>(`/manga/${encodeURIComponent(mangaId)}/chapters`);
    return normalizeChapterList(payload);
  } catch {
    return [];
  }
}

export async function loadChapter(mangaId: string, chapterId: string): Promise<Chapter | null> {
  try {
    const payload = await apiFetch<ChapterPayload>(
      `/manga/${encodeURIComponent(mangaId)}/chapters/${encodeURIComponent(chapterId)}`,
    );
    return normalizeChapterPayload(payload, 0);
  } catch {
    const chapters = await loadChapters(mangaId);
    return chapters.find((chapter) => chapter.id === chapterId) ?? null;
  }
}

export async function searchCatalog(query: string): Promise<CatalogManga[]> {
  const trimmed = query.trim();
  try {
    const payload = await apiFetch<{ results: Manga[]; count: number }>(`/manga?q=${encodeURIComponent(trimmed)}&limit=100`);
    return payload.results.map(mapManga);
  } catch {
    return [];
  }
}

export async function loadGenres(): Promise<string[]> {
  const catalog = await loadCatalog();
  return genresFromCatalog(catalog);
}

export function genresFromCatalog(catalog: CatalogManga[]) {
  return Array.from(new Set(catalog.flatMap((item) => item.genres))).sort();
}

export function filterByGenre(catalog: CatalogManga[], slug: string) {
  const label = genresFromCatalog(catalog).find((genre) => genreSlug(genre) === slug);
  if (!label) return { label: null, items: [] as CatalogManga[] };
  return { label, items: catalog.filter((manga) => manga.genres.includes(label)) };
}

export function latestUpdates(catalog: CatalogManga[]) {
  return [...catalog].sort((a, b) => b.totalChapters - a.totalChapters);
}

export function popularTitles(catalog: CatalogManga[]) {
  return [...catalog].sort((a, b) => {
    if (b.totalChapters !== a.totalChapters) return b.totalChapters - a.totalChapters;
    return a.title.localeCompare(b.title);
  });
}

export function relatedManga(manga: CatalogManga, catalog: CatalogManga[], limit = 5) {
  return catalog
    .filter((item) => item.slug !== manga.slug && item.genres.some((genre) => manga.genres.includes(genre)))
    .slice(0, limit);
}

export function libraryEntryToManga(entry: LibraryEntry, index = 0): CatalogManga {
  const manga: Manga = {
    id: entry.manga_id,
    title: entry.title,
    author: entry.author,
    genres: entry.genres,
    status: entry.reading_status,
    total_chapters: entry.total_chapters,
    description: `Saved in your library. Current chapter: ${entry.current_chapter}.`,
    cover_url: entry.cover_url,
    source_provider: entry.source_provider,
    source_url: entry.source_url,
    rights_status: entry.rights_status,
    cover_large_url: entry.cover_url,
  };
  const mapped = mapManga(manga, index);
  return {
    ...mapped,
    chapters: [],
  };
}

function mapManga(manga: Manga, index = 0): CatalogManga {
  const totalChapters = manga.total_chapters || 0;
  return {
    slug: manga.id,
    title: manga.title,
    author: manga.author,
    artist: manga.author,
    genres: manga.genres,
    status: normalizeStatus(manga.status),
    language: "Metadata",
    year: null,
    rating: "",
    views: "",
    description: manga.description,
    color: `${hashHue(manga.id || manga.title, index)}deg`,
    coverUrl: safeImageURL(manga.cover_large_url || manga.cover_url),
    sourceProvider: manga.source_provider || "MangaHub API",
    sourceUrl: manga.source_url || `/manga/${manga.id}`,
    rightsStatus: manga.rights_status || "metadata-only",
    totalChapters,
    chapters: [],
    coverLargeUrl: safeImageURL(manga.cover_large_url || manga.cover_url),
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

export function safeImageURL(value?: string) {
  const trimmed = value?.trim() ?? "";
  if (!trimmed) return "";
  const absolute = trimmed.startsWith("/") && !trimmed.startsWith("//") ? `${API_BASE.replace(/\/$/, "")}${trimmed}` : trimmed;
  return validImageURL(absolute) ? absolute : "";
}

export function validImageURL(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed) return false;

  try {
    const url = new URL(trimmed);
    if (url.protocol !== "http:" && url.protocol !== "https:") return false;
    return !/\/media\/covers\/[^/?#]+\.svg$/i.test(url.pathname);
  } catch {
    return false;
  }
}

type ChapterListPayload =
  | MangaChapter[]
  | {
      chapters?: MangaChapter[];
      results?: MangaChapter[];
      data?: MangaChapter[];
      count?: number;
    };

type ChapterPayload =
  | MangaChapter
  | {
      chapter?: MangaChapter;
      result?: MangaChapter;
      data?: MangaChapter;
    };

function normalizeChapterList(payload: ChapterListPayload): Chapter[] {
  const raw = Array.isArray(payload) ? payload : payload.chapters ?? payload.results ?? payload.data ?? [];
  return raw.map(normalizeChapter).sort((a, b) => b.number - a.number);
}

function normalizeChapterPayload(payload: ChapterPayload, index: number): Chapter | null {
  let raw: MangaChapter | undefined;
  if ("chapter" in payload || "result" in payload || "data" in payload) {
    raw = payload.chapter ?? payload.result ?? payload.data;
  } else {
    raw = payload as MangaChapter;
  }
  return raw ? normalizeChapter(raw, index) : null;
}

function normalizeChapter(raw: MangaChapter, index: number): Chapter {
  const number = numeric(raw.chapter_number ?? raw.number, index + 1);
  const pageUrls = extractPageUrls(raw);
  const pageCount = pageUrls.length || numeric(raw.page_count ?? raw.pages_count ?? (typeof raw.pages === "number" ? raw.pages : 0), 0);
  return {
    id: String(raw.id || number),
    title: raw.title || `Chapter ${number}`,
    number,
    updatedAt: raw.updated_at || raw.created_at || raw.publish_status || raw.status || "Uploaded chapter",
    pages: pageCount,
    pageUrls,
    publishStatus: raw.publish_status || raw.status || "published",
  };
}

function extractPageUrls(chapter: MangaChapter) {
  const fromNamedUrls = [...(chapter.page_urls ?? []), ...(chapter.image_urls ?? [])]
    .map(normalizePageURL)
    .filter(Boolean);
  if (fromNamedUrls.length > 0) return fromNamedUrls;
  if (!Array.isArray(chapter.pages)) return [];

  return chapter.pages
    .map((page) => {
      if (typeof page === "string") return page;
      return page.url ?? page.image_url ?? page.file_url ?? page.src ?? "";
    })
    .map(normalizePageURL)
    .filter(Boolean);
}

function normalizePageURL(value?: string) {
  const trimmed = value?.trim();
  if (!trimmed) return "";
  if (trimmed.startsWith("/") && !trimmed.startsWith("//")) {
    return `${API_BASE.replace(/\/$/, "")}${trimmed}`;
  }
  return validImageURL(trimmed) ? trimmed : "";
}

function numeric(value: unknown, fallback: number) {
  const parsed = typeof value === "number" ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function normalizeStatus(status: string): CatalogManga["status"] {
  const normalized = status.toLowerCase();
  if (normalized.includes("complete") || normalized === "finished") return "Completed";
  if (normalized.includes("hiatus")) return "Hiatus";
  if (normalized.includes("cancel")) return "Cancelled";
  if (normalized.includes("not-yet")) return "Not Yet Released";
  return "Ongoing";
}

function hashHue(value: string, index: number) {
  let hash = index * 37 + 12;
  for (const char of value) hash = (hash * 31 + char.charCodeAt(0)) % 360;
  return hash;
}
