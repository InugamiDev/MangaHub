export const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export type Manga = {
  id: string;
  title: string;
  author: string;
  genres: string[];
  status: string;
  total_chapters: number;
  description: string;
  cover_url: string;
  source_provider: string;
  source_url: string;
  rights_status: string;
  cover_large_url?: string;
  banner_url?: string;
  start_date?: string;
  end_date?: string;
  format?: string;
  volumes?: number;
  mean_score?: number;
  average_score?: number;
  popularity?: number;
  favourites?: number;
  country_of_origin?: string;
  is_licensed?: boolean;
  synonyms?: string[];
  tags?: MangaTag[];
  relations?: MangaRelation[];
  recommendations?: MangaRecommendation[];
  characters?: MangaCredit[];
  staff?: MangaCredit[];
  external_links?: MangaExternalLink[];
  rankings?: MangaRanking[];
};

export type MangaTag = {
  id: number;
  name: string;
  description?: string;
  rank: number;
  is_media_spoiler: boolean;
  is_general_spoiler: boolean;
};

export type MangaRelation = {
  id: number;
  relation_type: string;
  title: string;
  format?: string;
  type?: string;
  status?: string;
  cover_url?: string;
  banner_url?: string;
};

export type MangaRecommendation = {
  id: number;
  rating: number;
  user_rating?: string;
  title: string;
  format?: string;
  type?: string;
  status?: string;
  cover_url?: string;
  banner_url?: string;
};

export type MangaCredit = {
  id: number;
  name: string;
  role?: string;
  image_url?: string;
  language?: string;
};

export type MangaExternalLink = {
  id: number;
  site: string;
  url: string;
  type?: string;
  language?: string;
  color?: string;
  icon?: string;
  notes?: string;
};

export type MangaRanking = {
  id: number;
  rank: number;
  type: string;
  format?: string;
  year?: number;
  season?: string;
  all_time: boolean;
  context: string;
};

export type LibraryEntry = {
  manga_id: string;
  title: string;
  author: string;
  genres: string[];
  reading_status: string;
  current_chapter: number;
  total_chapters: number;
  cover_url: string;
  source_provider: string;
  source_url: string;
  rights_status: string;
  updated_at: string;
};

export type MangaChapterPage = {
  id?: string;
  url?: string;
  image_url?: string;
  file_url?: string;
  src?: string;
  page_number?: number;
  number?: number;
  order?: number;
};

export type MangaChapter = {
  id?: string;
  manga_id?: string;
  title?: string;
  chapter_number?: number;
  number?: number;
  publish_status?: string;
  status?: string;
  updated_at?: string;
  created_at?: string;
  page_count?: number;
  pages_count?: number;
  pages?: number | string[] | MangaChapterPage[];
  page_urls?: string[];
  image_urls?: string[];
};

export async function apiFetch<T>(
  path: string,
  options: RequestInit & { token?: string; adminToken?: string } = {},
): Promise<T> {
  const { token, adminToken, headers: initHeaders, body, ...fetchOptions } = options;
  const headers = new Headers(initHeaders);
  const hasFormBody = typeof FormData !== "undefined" && body instanceof FormData;
  if (!hasFormBody && body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  if (adminToken) {
    headers.set("X-Admin-Sync-Token", adminToken);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...fetchOptions,
    body,
    headers,
    cache: "no-store",
  });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error ?? `Request failed with ${response.status}`);
  }
  return payload as T;
}
