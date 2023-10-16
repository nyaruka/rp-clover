package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

type migration struct {
	version     int
	description string
	sql         string
}

var (
	migrations = []migration{
		{
			version:     1,
			description: "install migrations table",
			sql: `
			CREATE TABLE migrations (
				version INT unique NOT NULL,
				applied_on TIMESTAMP WITH TIME ZONE NOT NULL
			)`,
		},
		{
			version:     2,
			description: "install channels table",
			sql: `
			CREATE TABLE channels (
				name TEXT NOT NULL,
				uuid UUID NOT NULL UNIQUE,
				url TEXT NOT NULL,
				keywords TEXT[] NULL 
			)`,
		},
		{
			version:     3,
			description: "install interchanges table",
			sql: `
			CREATE TABLE interchanges (
				name TEXT NOT NULL UNIQUE,
				uuid UUID NOT NULL UNIQUE,
				country VARCHAR(2) NOT NULL,
				scheme VARCHAR(32) NOT NULL,
				default_channel_uuid UUID REFERENCES channels(uuid) INITIALLY DEFERRED NOT NULL
			)`,
		},
		{
			version:     4,
			description: "add interchange_uuid field",
			sql: `
			ALTER TABLE channels ADD COLUMN interchange_uuid UUID REFERENCES interchanges(uuid) ON DELETE CASCADE INITIALLY DEFERRED NOT NULL
			`,
		},
		{
			version:     5,
			description: "install urn_mappings table",
			sql: `
			CREATE TABLE urn_mappings (
				urn VARCHAR(1024) NOT NULL,
				interchange_uuid UUID REFERENCES interchanges(uuid) ON DELETE CASCADE NOT NULL,
				channel_uuid UUID REFERENCES channels(uuid) ON DELETE CASCADE NOT NULL
			)`,
		},
		{
			version:     6,
			description: "install mappings index",
			sql: `
			CREATE UNIQUE INDEX urn_mappings_idx ON urn_mappings(
				urn, 
				interchange_uuid
			)`,
		},
	}
)

const (
	latestMigrationSQL = `
	SELECT MAX(version) FROM migrations
	`

	insertMigration = `
	INSERT INTO migrations(version, applied_on) 
	VALUES($1, NOW())
	`
)

func getVersion(ctx context.Context, db *sqlx.DB) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()

	// get our latest version,
	version := 0
	err := db.GetContext(ctx, &version, latestMigrationSQL)

	// is our table missing? if so, we are version 0
	if err != nil && strings.Contains(err.Error(), "does not exist") {
		return 0, nil
	}

	// table exists, query current version
	return version, err
}

func apply(ctx context.Context, db *sqlx.DB, mig migration) error {
	log := slog.With(
		"version", mig.version,
		"description", mig.description,
	)

	log.Info("applying migration")

	_, err := db.ExecContext(ctx, mig.sql)
	if err != nil {
		log.Error("error applying migration", "error", err)
		return err
	}

	_, err = db.ExecContext(ctx, insertMigration, mig.version)
	if err != nil {
		log.Error("error inserting migration record", "error", err)
		return err
	}

	return nil
}

// Migrate installs any missing migrations for the passed in DB
func Migrate(ctx context.Context, db *sqlx.DB) error {
	version, err := getVersion(ctx, db)
	if err != nil {
		slog.Error("unable to get current db migration state", "error", err)
		return err
	}

	slog.Info("database at migration", "version", version)

	if version > migrations[len(migrations)-1].version {
		slog.Error(fmt.Sprintf("db version: %d is greater than max migration: %d", version, migrations[len(migrations)-1].version))
	}

	for _, mig := range migrations[version:] {
		err := apply(ctx, db, mig)
		if err != nil {
			return err
		}
	}

	return nil
}
