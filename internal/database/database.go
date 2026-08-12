// Package database handles SQLite connection and migrations.
package database

import (
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/mattn/go-sqlite3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// DB wraps the SQL database connection.
type DB struct {
	Conn   *sql.DB
	logger *slog.Logger
}

// New creates a new database connection and runs migrations.
// The dbPath is the file path for the SQLite database.
func New(dbPath string, logger *slog.Logger) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Verify connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	db := &DB{
		Conn:   conn,
		logger: logger,
	}

	// Run migrations
	if err := db.migrate(); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	logger.Info("Database ready", "path", dbPath)
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.Conn.Close()
}

// migrate runs all SQL migration files in order.
func (db *DB) migrate() error {
	// Create migrations tracking table
	_, err := db.Conn.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	// Read migration files
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	// Sort by filename to ensure order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()

		// Check if already applied
		var count int
		err := db.Conn.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", filename).Scan(&count)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}
		if count > 0 {
			continue
		}

		// Read and execute migration
		content, err := migrationsFS.ReadFile("migrations/" + filename)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		_, err = db.Conn.Exec(string(content))
		if err != nil {
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		// Record migration
		_, err = db.Conn.Exec(
			"INSERT INTO schema_migrations (filename, applied_at) VALUES (?, datetime('now'))",
			filename,
		)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		db.logger.Info("Migration applied", "file", filename)
	}

	return nil
}
