package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationsDir = "migrations"

// Run запускает миграции только если установлена MIGRATE=1/true/yes.
func Run(ctx context.Context, pool *pgxpool.Pool) error {
	v := os.Getenv("MIGRATE")
	if v == "" {
		return nil // миграции выключены
	}
	v = strings.ToLower(strings.TrimSpace(v))
	if v != "1" && v != "true" && v != "yes" {
		return nil // любые другие значения считаем "выключено"
	}

	if err := ensureSchemaMigrations(ctx, pool); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("read migrations dir %q: %w", migrationsDir, err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		files = append(files, filepath.Join(migrationsDir, name))
	}
	sort.Strings(files)

	for _, path := range files {
		name := filepath.Base(path)

		applied, err := isApplied(ctx, pool, name)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		sqlText := string(sqlBytes)

		if err := applyMigration(ctx, pool, name, sqlText); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}

	return nil
}

func ensureSchemaMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	const q = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    name       text PRIMARY KEY,
    applied_at timestamptz NOT NULL
);`
	_, err := pool.Exec(ctx, q)
	return err
}

func isApplied(ctx context.Context, pool *pgxpool.Pool, name string) (bool, error) {
	const q = `SELECT 1 FROM schema_migrations WHERE name = $1`
	row := pool.QueryRow(ctx, q, name)
	var tmp int
	if err := row.Scan(&tmp); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func applyMigration(ctx context.Context, pool *pgxpool.Pool, name, sqlText string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, sqlText); err != nil {
		return fmt.Errorf("exec migration sql: %w", err)
	}

	const insertQ = `INSERT INTO schema_migrations(name, applied_at) VALUES ($1, $2)`
	if _, err := tx.Exec(ctx, insertQ, name, time.Now().UTC()); err != nil {
		return fmt.Errorf("insert schema_migrations: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
