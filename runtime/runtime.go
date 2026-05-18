package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/nyaruka/rp-clover/migrations"
)

// Runtime holds the configuration and live service handles used across the clover server.
type Runtime struct {
	Config *Config
	DB     *sqlx.DB
}

// NewRuntime opens the database, verifies connectivity, runs migrations, and returns a ready Runtime.
func NewRuntime(cfg *Config) (*Runtime, error) {
	rt := &Runtime{Config: cfg}

	db, err := sqlx.Open("postgres", cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("error opening db: %w", err)
	}
	db.SetMaxOpenConns(4)
	rt.DB = db

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	if err := rt.DB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("error pinging db: %w", err)
	}
	if err := migrations.Migrate(ctx, db); err != nil {
		return nil, fmt.Errorf("error running migrations: %w", err)
	}
	return rt, nil
}
