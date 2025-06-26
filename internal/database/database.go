package database

import (
	"context"
	"fmt"

	"github.com/USSTM/cv-backend/generated/db"
	"github.com/USSTM/cv-backend/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func New(cfg *config.DatabaseConfig) (*Database, error) {
	pool, err := pgxpool.New(context.Background(), cfg.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Activate and test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	queries := db.New(pool)

	return &Database{
		pool:    pool,
		queries: queries,
	}, nil
}

func (d *Database) Close() {
	if d.pool != nil {
		d.pool.Close()
	}
}

func (d *Database) Queries() *db.Queries {
	return d.queries
}

func (d *Database) Pool() *pgxpool.Pool {
	return d.pool
}
