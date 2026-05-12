package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mangahub/services/api/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

type Dialect string

const (
	DialectPostgres Dialect = "postgres"
	DialectSQLite   Dialect = "sqlite"
)

type Store struct {
	*sql.DB
	Dialect Dialect
}

func Open(path string) (*Store, error) {
	return OpenConfigured("", path)
}

func OpenConfigured(databaseURL, sqlitePath string) (*Store, error) {
	if isPostgresURL(databaseURL) {
		config, err := pgx.ParseConfig(normalizePostgresURL(databaseURL))
		if err != nil {
			return nil, err
		}
		config.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
		db := stdlib.OpenDB(*config)
		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(30 * time.Minute)
		store := &Store{DB: db, Dialect: DialectPostgres}
		return store, store.Ping()
	}

	if sqlitePath == "" {
		sqlitePath = "mangahub.db"
	}
	db, err := sql.Open("sqlite3", sqlitePath+"?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{DB: db, Dialect: DialectSQLite}
	return store, store.Ping()
}

func (s *Store) Exec(query string, args ...any) (sql.Result, error) {
	return s.DB.Exec(s.Rebind(query), args...)
}

func (s *Store) Query(query string, args ...any) (*sql.Rows, error) {
	return s.DB.Query(s.Rebind(query), args...)
}

func (s *Store) QueryRow(query string, args ...any) *sql.Row {
	return s.DB.QueryRow(s.Rebind(query), args...)
}

func (s *Store) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return s.DB.ExecContext(ctx, s.Rebind(query), args...)
}

func (s *Store) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return s.DB.QueryContext(ctx, s.Rebind(query), args...)
}

func (s *Store) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return s.DB.QueryRowContext(ctx, s.Rebind(query), args...)
}

func (s *Store) Rebind(query string) string {
	if s.Dialect != DialectPostgres {
		return query
	}
	var builder strings.Builder
	index := 1
	for _, r := range query {
		if r == '?' {
			builder.WriteByte('$')
			builder.WriteString(intToString(index))
			index++
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

func Migrate(store *Store) error {
	_, err := store.Exec(`
CREATE TABLE IF NOT EXISTS users (
	id TEXT PRIMARY KEY,
	username TEXT UNIQUE NOT NULL,
	email TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'reader',
	created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS manga (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	author TEXT NOT NULL,
	genres TEXT NOT NULL,
	status TEXT NOT NULL,
	total_chapters INTEGER NOT NULL,
	description TEXT NOT NULL,
	cover_url TEXT NOT NULL,
	source_provider TEXT NOT NULL DEFAULT 'MangaHub seed',
	source_url TEXT NOT NULL DEFAULT '',
	rights_status TEXT NOT NULL DEFAULT 'metadata-only'
);

CREATE TABLE IF NOT EXISTS chapters (
	id TEXT PRIMARY KEY,
	manga_id TEXT NOT NULL,
	title TEXT NOT NULL,
	chapter_number REAL NOT NULL,
	page_urls TEXT NOT NULL,
	page_count INTEGER NOT NULL DEFAULT 0,
	publish_status TEXT NOT NULL DEFAULT 'draft',
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_progress (
	user_id TEXT NOT NULL,
	manga_id TEXT NOT NULL,
	current_chapter INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (user_id, manga_id),
	FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
	FOREIGN KEY (manga_id) REFERENCES manga(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_manga_title ON manga(title);
CREATE INDEX IF NOT EXISTS idx_manga_author ON manga(author);
CREATE INDEX IF NOT EXISTS idx_chapters_manga ON chapters(manga_id);
CREATE INDEX IF NOT EXISTS idx_progress_user ON user_progress(user_id);
`)
	if err != nil {
		return err
	}
	if err := ensureColumn(store, "manga", "source_provider", "ALTER TABLE manga ADD COLUMN source_provider TEXT NOT NULL DEFAULT 'MangaHub seed'"); err != nil {
		return err
	}
	if err := ensureColumn(store, "manga", "source_url", "ALTER TABLE manga ADD COLUMN source_url TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(store, "manga", "rights_status", "ALTER TABLE manga ADD COLUMN rights_status TEXT NOT NULL DEFAULT 'metadata-only'"); err != nil {
		return err
	}
	return ensureColumn(store, "users", "role", "ALTER TABLE users ADD COLUMN role TEXT NOT NULL DEFAULT 'reader'")
}

func SeedManga(store *Store, seedPath string) error {
	payload, err := readSeed(seedPath)
	if err != nil {
		return err
	}
	var entries []models.Manga
	if err := json.Unmarshal(payload, &entries); err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("seed file has no manga")
	}
	return upsertManga(store, entries, true)
}

func UpsertManga(store *Store, entries []models.Manga) error {
	return upsertManga(store, entries, false)
}

func upsertManga(store *Store, entries []models.Manga, pruneStaleSeed bool) error {
	tx, err := store.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(store.Rebind(`
INSERT INTO manga (id, title, author, genres, status, total_chapters, description, cover_url, source_provider, source_url, rights_status)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	title = excluded.title,
	author = excluded.author,
	genres = excluded.genres,
	status = excluded.status,
	total_chapters = excluded.total_chapters,
	description = excluded.description,
	cover_url = excluded.cover_url,
	source_provider = excluded.source_provider,
	source_url = excluded.source_url,
	rights_status = excluded.rights_status`))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, manga := range entries {
		if manga.SourceProvider == "" {
			manga.SourceProvider = "MangaHub seed"
		}
		if manga.RightsStatus == "" {
			manga.RightsStatus = "metadata-only"
		}
		manga.CoverURL = models.SanitizeCoverURL(manga.CoverURL)
		genres, err := json.Marshal(manga.Genres)
		if err != nil {
			return err
		}
		if _, err := stmt.Exec(manga.ID, manga.Title, manga.Author, string(genres), manga.Status, manga.TotalChapters, manga.Description, manga.CoverURL, manga.SourceProvider, manga.SourceURL, manga.RightsStatus); err != nil {
			return err
		}
	}
	if pruneStaleSeed {
		if err := pruneStaleSeedManga(tx, store, entries); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func pruneStaleSeedManga(tx *sql.Tx, store *Store, entries []models.Manga) error {
	if len(entries) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(entries))
	args := make([]any, 0, len(entries))
	for _, manga := range entries {
		placeholders = append(placeholders, "?")
		args = append(args, manga.ID)
	}
	query := `
DELETE FROM manga
WHERE rights_status = 'metadata-only'
  AND source_provider IN ('MangaHub seed', 'AniList metadata')
  AND id NOT IN (` + strings.Join(placeholders, ",") + `)`
	_, err := tx.Exec(store.Rebind(query), args...)
	return err
}

func ensureColumn(store *Store, table, column, ddl string) error {
	exists, err := columnExists(store, table, column)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	_, err = store.Exec(ddl)
	return err
}

func columnExists(store *Store, table, column string) (bool, error) {
	if store.Dialect == DialectPostgres {
		var exists bool
		err := store.QueryRow(`
SELECT EXISTS (
	SELECT 1
	FROM information_schema.columns
	WHERE table_schema = 'public' AND table_name = ? AND column_name = ?
)`, table, column).Scan(&exists)
		return exists, err
	}

	rows, err := store.DB.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typ string
		var notNull int
		var defaultValue any
		var primaryKey int
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func readSeed(seedPath string) ([]byte, error) {
	candidates := []string{
		seedPath,
		filepath.Join("services", "api", seedPath),
		filepath.Join("..", "..", "services", "api", seedPath),
	}
	for _, path := range candidates {
		if payload, err := os.ReadFile(path); err == nil {
			return payload, nil
		}
	}
	return nil, os.ErrNotExist
}

func isPostgresURL(raw string) bool {
	return strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://")
}

func normalizePostgresURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	values := parsed.Query()
	if values.Get("sslmode") == "" {
		values.Set("sslmode", "require")
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func intToString(value int) string {
	const digits = "0123456789"
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = digits[value%10]
		value /= 10
	}
	return string(buf[i:])
}
