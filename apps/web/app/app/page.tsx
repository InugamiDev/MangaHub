"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertCircle,
  BookOpen,
  Database,
  Library,
  LogIn,
  Plus,
  RefreshCcw,
  Search,
  Server,
  ShieldCheck,
  Star,
  Terminal,
  Wifi,
} from "lucide-react";
import { API_BASE, apiFetch, type LibraryEntry, type Manga } from "@/lib/api";
import { hasAdminCredentials, readAdminCredentials } from "@/components/admin/AdminTokenField";
import { safeImageURL } from "@/lib/catalogData";

type AuthState = {
  token: string;
  username: string;
  role: string;
};

type HealthPayload = {
  status: string;
  database: { status: string; dialect: string };
  services: Record<string, string>;
};

type LegalSource = {
  id: string;
  name: string;
  kind: string;
  rights_status: string;
  endpoint: string;
};

type CommandEntry = {
  command: string;
  output: string;
  ok: boolean;
  durationMs: number;
};

type CommandMetrics = {
  total: number;
  success: number;
  failure: number;
  avgMs: number;
  lastStatus: string;
};

const commandHelp = [
  "health",
  "sources",
  "me",
  "manga search <query>",
  "manga info <id>",
  "source anilist <query>",
  "source mangadex <query>",
  "sync anilist <status>",
  "sync mangadex <status>",
  "login <username> <password>",
  "register <username> <email> <password>",
  "library list",
  "library add <manga_id>",
  "progress <manga_id> <chapter>",
  "clear",
];

export default function AppPage() {
  const [auth, setAuth] = useState<AuthState | null>(null);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [query, setQuery] = useState("one");
  const [results, setResults] = useState<Manga[]>([]);
  const [library, setLibrary] = useState<LibraryEntry[]>([]);
  const [health, setHealth] = useState<HealthPayload | null>(null);
  const [sources, setSources] = useState<LegalSource[]>([]);
  const [message, setMessage] = useState("Connect an account to save library entries and broadcast progress updates.");
  const [loading, setLoading] = useState(false);
  const [command, setCommand] = useState("health");
  const [commandLog, setCommandLog] = useState<CommandEntry[]>([
    {
      command: "system",
      output: "MangaHub site CLI is ready. Type help to list commands.",
      ok: true,
      durationMs: 0,
    },
  ]);
  const [commandMetrics, setCommandMetrics] = useState<CommandMetrics>({
    total: 0,
    success: 0,
    failure: 0,
    avgMs: 0,
    lastStatus: "idle",
  });

  const stats = useMemo(() => {
    const chapters = library.reduce((sum, item) => sum + item.current_chapter, 0);
    const reading = library.filter((item) => item.reading_status === "reading").length;
    return { chapters, reading, total: library.length };
  }, [library]);

  const refreshHealth = useCallback(async () => {
    try {
      const [healthPayload, sourcePayload] = await Promise.all([
        apiFetch<HealthPayload>("/health"),
        apiFetch<{ sources: LegalSource[] }>("/sources"),
      ]);
      setHealth(healthPayload);
      setSources(sourcePayload.sources);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Health check failed");
    }
  }, []);

  const searchManga = useCallback(async () => {
    setLoading(true);
    try {
      const payload = await apiFetch<{ results: Manga[] }>(`/manga?q=${encodeURIComponent(query)}&limit=12`);
      setResults(payload.results);
      setMessage(`Loaded ${payload.results.length} catalog records from ${API_BASE}.`);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Search failed");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => {
    void refreshHealth();
    void searchManga();
  }, [refreshHealth, searchManga]);

  async function register() {
    setLoading(true);
    try {
      await apiFetch("/auth/register", {
        method: "POST",
        body: JSON.stringify({ username, email, password }),
      });
      setMessage("Account created. Logging in next.");
      await login();
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Registration failed");
    } finally {
      setLoading(false);
    }
  }

  async function login() {
    setLoading(true);
    try {
      const payload = await apiFetch<{ token: string; user: { username: string; role: string } }>("/auth/login", {
        method: "POST",
        body: JSON.stringify({ username, password }),
      });
      localStorage.setItem("mangahub_token", payload.token);
      localStorage.setItem("mangahub_user", JSON.stringify(payload.user));
      setAuth({ token: payload.token, username: payload.user.username, role: payload.user.role });
      setMessage(`Logged in as ${payload.user.username}.`);
      await loadLibrary(payload.token);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Login failed");
    } finally {
      setLoading(false);
    }
  }

  async function loadLibrary(token = auth?.token) {
    if (!token) return;
    try {
      const payload = await apiFetch<{ entries: LibraryEntry[] }>("/users/library", { token });
      setLibrary(payload.entries);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Library load failed");
    }
  }

  async function addToLibrary(manga: Manga) {
    if (!auth) {
      setMessage("Log in before adding manga.");
      return;
    }
    setLoading(true);
    try {
      await apiFetch("/users/library", {
        method: "POST",
        token: auth.token,
        body: JSON.stringify({ manga_id: manga.id, status: "reading", current_chapter: manga.total_chapters > 0 ? 1 : 0 }),
      });
      setMessage(`${manga.title} added to your reading list.`);
      await loadLibrary(auth.token);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Add to library failed");
    } finally {
      setLoading(false);
    }
  }

  async function updateProgress(entry: LibraryEntry) {
    if (!auth) return;
    const nextChapter = Math.min(entry.current_chapter + 1, entry.total_chapters);
    setLoading(true);
    try {
      await apiFetch("/users/progress", {
        method: "PUT",
        token: auth.token,
        body: JSON.stringify({ manga_id: entry.manga_id, chapter: nextChapter, status: entry.reading_status }),
      });
      setMessage(`${entry.title} updated to chapter ${nextChapter}. Progress was broadcast to connected TCP clients.`);
      await loadLibrary(auth.token);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Progress update failed");
    } finally {
      setLoading(false);
    }
  }

  function recordMetric(ok: boolean, durationMs: number, label: string) {
    setCommandMetrics((current) => {
      const total = current.total + 1;
      return {
        total,
        success: current.success + (ok ? 1 : 0),
        failure: current.failure + (ok ? 0 : 1),
        avgMs: Math.round((current.avgMs * current.total + durationMs) / total),
        lastStatus: `${ok ? "ok" : "error"}: ${label}`,
      };
    });
  }

  async function runMeasured<T>(label: string, action: () => Promise<T>) {
    const started = performance.now();
    try {
      const payload = await action();
      const durationMs = Math.round(performance.now() - started);
      recordMetric(true, durationMs, label);
      return { payload, durationMs };
    } catch (error) {
      const durationMs = Math.round(performance.now() - started);
      recordMetric(false, durationMs, label);
      throw Object.assign(error instanceof Error ? error : new Error("Command failed"), { durationMs });
    }
  }

  async function executeCommand() {
    const raw = command.trim();
    if (!raw) return;
    if (raw === "clear") {
      setCommandLog([]);
      setCommand("");
      return;
    }

    const started = performance.now();
    try {
      const output = await routeCommand(raw);
      const durationMs = typeof output.durationMs === "number" ? output.durationMs : Math.round(performance.now() - started);
      setCommandLog((items) => [
        { command: raw, output: formatCommandOutput(output.payload), ok: true, durationMs },
        ...items,
      ].slice(0, 12));
      setCommand("");
    } catch (error) {
      const durationMs = Number((error as { durationMs?: number }).durationMs ?? Math.round(performance.now() - started));
      setCommandLog((items) => [
        { command: raw, output: error instanceof Error ? error.message : "Command failed", ok: false, durationMs },
        ...items,
      ].slice(0, 12));
    }
  }

  async function routeCommand(raw: string): Promise<{ payload: unknown; durationMs: number }> {
    const parts = raw.split(/\s+/);
    const [root, sub] = parts;
    if (root === "help") {
      return { payload: { commands: commandHelp }, durationMs: 0 };
    }
    if (root === "health") {
      return runMeasured("GET /health", async () => {
        const payload = await apiFetch<HealthPayload>("/health");
        setHealth(payload);
        return payload;
      });
    }
    if (root === "sources") {
      return runMeasured("GET /sources", async () => {
        const payload = await apiFetch<{ sources: LegalSource[] }>("/sources");
        setSources(payload.sources);
        return payload;
      });
    }
    if (root === "me") {
      if (!auth) throw new Error("Login required for me");
      return runMeasured("GET /users/me", () => apiFetch("/users/me", { token: auth.token }));
    }
    if (root === "manga" && sub === "search") {
      const q = parts.slice(2).join(" ").trim() || query;
      return runMeasured("GET /manga", async () => {
        const payload = await apiFetch<{ results: Manga[]; count: number }>(`/manga?q=${encodeURIComponent(q)}&limit=12`);
        setQuery(q);
        setResults(payload.results);
        return {
          count: payload.count,
          results: payload.results.map((manga) => ({
            id: manga.id,
            title: manga.title,
            cover: Boolean(manga.cover_url),
            score: manga.average_score || manga.mean_score || null,
          })),
        };
      });
    }
    if (root === "manga" && sub === "info") {
      const id = parts[2];
      if (!id) throw new Error("Usage: manga info <id>");
      return runMeasured(`GET /manga/${id}`, () => apiFetch<Manga>(`/manga/${encodeURIComponent(id)}`));
    }
    if (root === "source" && (sub === "anilist" || sub === "mangadex")) {
      const q = parts.slice(2).join(" ").trim() || query;
      return runMeasured(`GET /sources/${sub}/search`, async () => {
        const payload = await apiFetch<{ results: Manga[]; count: number }>(`/sources/${sub}/search?q=${encodeURIComponent(q)}&limit=8`);
        return {
          count: payload.count,
          results: payload.results.map((manga) => ({
            id: manga.id,
            title: manga.title,
            status: manga.status,
            cover: Boolean(manga.cover_url),
            source: manga.source_provider,
          })),
        };
      });
    }
    if (root === "sync" && (sub === "anilist" || sub === "mangadex")) {
      const adminCredentials = readAdminCredentials();
      if (!hasAdminCredentials(adminCredentials)) throw new Error("Admin role or server admin token required for sync");
      const statuses = parts.slice(2).length > 0 ? parts.slice(2) : ["ongoing", "hiatus"];
      return runMeasured(`POST /admin/sync/${sub}`, () => apiFetch(`/admin/sync/${sub}`, {
        method: "POST",
        ...adminCredentials,
        body: JSON.stringify({ statuses, limit: sub === "mangadex" ? 50 : 35 }),
      }));
    }
    if (root === "login") {
      const [loginUsername, loginPassword] = [parts[1], parts[2]];
      if (!loginUsername || !loginPassword) throw new Error("Usage: login <username> <password>");
      return runMeasured("POST /auth/login", async () => {
        const payload = await apiFetch<{ token: string; user: { username: string; role: string } }>("/auth/login", {
          method: "POST",
          body: JSON.stringify({ username: loginUsername, password: loginPassword }),
        });
        localStorage.setItem("mangahub_token", payload.token);
        localStorage.setItem("mangahub_user", JSON.stringify(payload.user));
        setAuth({ token: payload.token, username: payload.user.username, role: payload.user.role });
        await loadLibrary(payload.token);
        return { user: payload.user, token: "redacted" };
      });
    }
    if (root === "register") {
      const [registerUsername, registerEmail, registerPassword] = [parts[1], parts[2], parts[3]];
      if (!registerUsername || !registerEmail || !registerPassword) throw new Error("Usage: register <username> <email> <password>");
      return runMeasured("POST /auth/register", () => apiFetch("/auth/register", {
        method: "POST",
        body: JSON.stringify({ username: registerUsername, email: registerEmail, password: registerPassword }),
      }));
    }
    if (root === "library" && sub === "list") {
      if (!auth) throw new Error("Login required for library list");
      return runMeasured("GET /users/library", async () => {
        const payload = await apiFetch<{ entries: LibraryEntry[]; count: number }>("/users/library", { token: auth.token });
        setLibrary(payload.entries);
        return payload;
      });
    }
    if (root === "library" && sub === "add") {
      if (!auth) throw new Error("Login required for library add");
      const id = parts[2];
      if (!id) throw new Error("Usage: library add <manga_id>");
      return runMeasured("POST /users/library", async () => {
        const payload = await apiFetch("/users/library", {
          method: "POST",
          token: auth.token,
          body: JSON.stringify({ manga_id: id, status: "reading", current_chapter: 0 }),
        });
        await loadLibrary(auth.token);
        return payload;
      });
    }
    if (root === "progress") {
      if (!auth) throw new Error("Login required for progress");
      const id = parts[1];
      const chapter = Number(parts[2]);
      if (!id || !Number.isFinite(chapter)) throw new Error("Usage: progress <manga_id> <chapter>");
      return runMeasured("PUT /users/progress", async () => {
        const payload = await apiFetch("/users/progress", {
          method: "PUT",
          token: auth.token,
          body: JSON.stringify({ manga_id: id, chapter, status: "reading" }),
        });
        await loadLibrary(auth.token);
        return payload;
      });
    }
    throw new Error(`Unknown command: ${raw}`);
  }

  const serviceEntries = Object.entries(health?.services ?? {});
  const featuredResult = results.find((manga) => manga.cover_url) ?? results[0];
  const featuredCoverURL = safeImageURL(featuredResult?.cover_url);
  const parallaxCovers = results
    .map((manga) => ({ manga, coverURL: safeImageURL(manga.cover_url) }))
    .filter((item) => item.coverURL)
    .slice(0, 10);

  return (
    <main className="app-wrap ops-console">
      <div className="ops-image-parallax" aria-hidden="true">
        {parallaxCovers.map(({ manga, coverURL }, index) => (
          <img src={coverURL} alt="" key={`${manga.id}-${index}`} />
        ))}
      </div>
      <div className="shell">
        <div className="section-head ops-head">
          <div>
            <span className="eyebrow">
              <Activity size={15} />
              Live MangaHub control surface
            </span>
            <h1 className="section-title">Operations Console</h1>
          </div>
          <p className="section-copy">
            Health, source adapters, catalog search, authenticated library writes, and TCP progress sync in one working surface.
          </p>
        </div>

        <section className="ops-status-grid">
          <article>
            <Database size={20} />
            <div>
              <strong>{health?.database.dialect ?? "database"}</strong>
              <span>{health?.database.status ?? "checking"}</span>
            </div>
          </article>
          {serviceEntries.map(([name, status]) => (
            <article key={name}>
              <Wifi size={20} />
              <div>
                <strong>{name}</strong>
                <span>{status}</span>
              </div>
            </article>
          ))}
        </section>

        <div className="ops-layout">
          <aside className="ops-sidebar">
            <section className="form-panel">
              <h3><Server size={17} /> Session</h3>
            <div className="form-stack">
              <div className="field">
                <label htmlFor="username">Username</label>
                <input id="username" value={username} onChange={(event) => setUsername(event.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="email">Email</label>
                <input id="email" value={email} onChange={(event) => setEmail(event.target.value)} />
              </div>
              <div className="field">
                <label htmlFor="password">Password</label>
                <input id="password" type="password" value={password} onChange={(event) => setPassword(event.target.value)} />
              </div>
              <button className="button-primary" type="button" onClick={register} disabled={loading}>
                <Plus size={17} /> Register
              </button>
              <button className="button-secondary" type="button" onClick={login} disabled={loading}>
                <LogIn size={17} /> Login
              </button>
              <p className="muted">{auth ? `Authenticated as ${auth.username} · ${auth.role}` : "Signed out"}</p>
            </div>
            </section>

            <section className="form-panel">
              <h3><ShieldCheck size={17} /> Source adapters</h3>
              <div className="source-mini-list">
                {sources.map((source) => (
                  <span key={source.id}>
                    <strong>{source.name}</strong>
                    <small>{source.kind} · {source.rights_status}</small>
                  </span>
                ))}
              </div>
              <button className="button-secondary reader-button" type="button" onClick={refreshHealth}>
                <RefreshCcw size={16} /> Refresh health
              </button>
            </section>
          </aside>

          <section className="ops-main">
            <div className="dashboard-grid">
              <Metric value={stats.total} label="library entries" />
              <Metric value={stats.reading} label="currently reading" />
              <Metric value={stats.chapters} label="chapters tracked" />
              <Metric value={results.length} label="catalog results" />
            </div>

            <section className="ops-hero-card">
              {featuredCoverURL ? <img src={featuredCoverURL} alt="" /> : null}
              <div>
                <span className="eyebrow"><BookOpen size={14} /> Catalog bridge</span>
                <h2>{featuredResult?.title ?? "Search the catalog"}</h2>
                <p>{featuredResult?.description ?? "Search MangaHub records enriched through the backend source adapters."}</p>
                {featuredResult ? (
                  <div className="meta-row">
                    <span><Star size={14} /> {featuredResult.average_score || featuredResult.mean_score || "AniList"} score</span>
                    <span>{featuredResult.total_chapters || "Unknown"} chapters</span>
                    <span>{featuredResult.source_provider}</span>
                  </div>
                ) : null}
              </div>
            </section>

            <section className="ops-panel">
              <div className="ops-panel-head">
                <div>
                  <h3><Terminal size={18} /> Site CLI</h3>
                  <p>Run real API commands from the browser and inspect latency, failures, and payloads.</p>
                </div>
              </div>
              <div className="cli-metrics">
                <Metric value={commandMetrics.total} label="commands" />
                <Metric value={commandMetrics.success} label="success" />
                <Metric value={commandMetrics.failure} label="failed" />
                <Metric value={commandMetrics.avgMs} label="avg ms" />
              </div>
              <div className="cli-command-row">
                <span>$</span>
                <input
                  value={command}
                  onChange={(event) => setCommand(event.target.value)}
                  onKeyDown={(event) => event.key === "Enter" && executeCommand()}
                  aria-label="MangaHub site CLI command"
                />
                <button className="button-primary reader-button" type="button" onClick={executeCommand}>
                  Run
                </button>
              </div>
              <div className="cli-help-grid">
                {commandHelp.slice(0, 8).map((item) => <button type="button" onClick={() => setCommand(item)} key={item}>{item}</button>)}
              </div>
              <p className="muted">Last status: {commandMetrics.lastStatus}</p>
              <div className="cli-log">
                {commandLog.map((entry, index) => (
                  <article className={entry.ok ? "ok" : "error"} key={`${entry.command}-${index}`}>
                    <header><span>$ {entry.command}</span><small>{entry.durationMs}ms</small></header>
                    <pre>{entry.output}</pre>
                  </article>
                ))}
              </div>
            </section>

            <section className="ops-panel">
              <div className="ops-panel-head">
                <div>
                  <h3>Catalog Search</h3>
                  <p>{loading ? "Loading from API" : message}</p>
                </div>
                <div className="ops-search">
                  <input id="query" value={query} onChange={(event) => setQuery(event.target.value)} onKeyDown={(event) => event.key === "Enter" && searchManga()} />
                  <button className="icon-button" type="button" aria-label="Search manga" onClick={searchManga}>
                    <Search size={18} />
                  </button>
                </div>
              </div>

              <div className="ops-result-grid">
                {results.map((manga) => {
                  const coverURL = safeImageURL(manga.cover_url);
                  return (
                    <article className="ops-manga-card" key={manga.id}>
                      {coverURL ? <img src={coverURL} alt="" /> : <span className="cover-tile">{manga.title.slice(0, 1)}</span>}
                      <div>
                        <strong>{manga.title}</strong>
                        <span>{manga.author}</span>
                        <small>{manga.genres.slice(0, 3).join(", ")}</small>
                      </div>
                      <button className="icon-button" type="button" aria-label={`Add ${manga.title}`} onClick={() => addToLibrary(manga)}>
                        <Plus size={18} />
                      </button>
                    </article>
                  );
                })}
              </div>
            </section>

            <section className="ops-panel">
              <div className="ops-panel-head">
                <div>
                  <h3>Authenticated Library</h3>
                  <p>{auth ? `Writing as ${auth.username}` : "Login to test protected routes"}</p>
                </div>
                <Library size={20} />
              </div>
              <div className="ops-library-list">
                {library.length === 0 ? (
                  <div className="ops-empty">
                    <AlertCircle size={18} />
                    <span>Add manga from the search results to populate this API-backed list.</span>
                  </div>
                ) : (
                  library.map((entry) => {
                    const coverURL = safeImageURL(entry.cover_url);
                    return (
                      <div className="manga-row" key={entry.manga_id}>
                        {coverURL ? <img className="row-cover" src={coverURL} alt="" /> : <span className="cover-tile">{entry.title.slice(0, 1)}</span>}
                        <span>
                          <strong>{entry.title}</strong>
                          <br />
                          <span className="muted">Chapter {entry.current_chapter}/{entry.total_chapters} · {entry.reading_status}</span>
                        </span>
                        <button className="icon-button" type="button" aria-label={`Update ${entry.title}`} onClick={() => updateProgress(entry)}>
                          <RefreshCcw size={18} />
                        </button>
                      </div>
                    );
                  })
                )}
              </div>
            </section>
          </section>
        </div>
      </div>
    </main>
  );
}

function formatCommandOutput(value: unknown) {
  if (typeof value === "string") return value;
  return JSON.stringify(value, (key, item) => key.toLowerCase().includes("token") ? "redacted" : item, 2);
}

function Metric({ value, label }: { value: number; label: string }) {
  return (
    <div className="metric-card">
      <h3>{value}</h3>
      <p>{label}</p>
    </div>
  );
}
