// Package database opens the PostgreSQL connection pool and runs the migrations.
package database

import (
	"context"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/brunohms/cat-n-rat/app/backend/migrations"
)

func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	// One span per query; a no-op when telemetry is disabled
	config.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName())
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// MigrateUp applies all pending migrations.
func MigrateUp(pool *pgxpool.Pool) error {
	return withMigrator(pool, func(m *migrate.Migrate) error {
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	})
}

// MigrateDown reverts the given number of migrations.
func MigrateDown(pool *pgxpool.Pool, steps int) error {
	return withMigrator(pool, func(m *migrate.Migrate) error {
		if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	})
}

// MigrationVersion returns the current schema version and whether the last migration failed halfway.
func MigrationVersion(pool *pgxpool.Pool) (version uint, dirty bool, err error) {
	err = withMigrator(pool, func(m *migrate.Migrate) error {
		version, dirty, err = m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			return nil
		}
		return err
	})
	return version, dirty, err
}

func withMigrator(pool *pgxpool.Pool, run func(*migrate.Migrate) error) error {
	source, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}
	// Releases the connection the migrator holds; without it pool.Close() blocks forever
	defer m.Close()

	if err := run(m); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
