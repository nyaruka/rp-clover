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

	// short timeout just to confirm connectivity
	pingCtx, cancelPing := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelPing()
	if err := rt.DB.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("error pinging db: %w", err)
	}

	// migrations get their own, more generous budget since they may wait on an
	// advisory lock held by another instance that is mid-migration during a deploy
	migrateCtx, cancelMigrate := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancelMigrate()
	if err := migrations.Migrate(migrateCtx, db); err != nil {
		return nil, fmt.Errorf("error running migrations: %w", err)
	}
	return rt, nil
}
