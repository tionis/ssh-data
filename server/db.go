package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
)

// sqlite db
type UserDB struct {
	db *sql.DB
}

var (
	migrations = []string{
		`CREATE TABLE data(
    			key TEXT PRIMARY KEY,
    			type TEXT NOT NULL DEFAULT 'string',
    			value TEXT NOT NULL DEFAULT '',
                validUntil INTEGER NOT NULL DEFAULT -1
        );`,
	}
)

func applyMigrations(db *sql.DB, ctx context.Context, logger *slog.Logger) error {
	for {
		query, err := db.Query("PRAGMA user_version;")
		if err != nil {
			return err
		}
		var version int
		hasValue := query.Next()
		if !hasValue {
			return fmt.Errorf("could not get user_version")
		}
		err = query.Scan(&version)
		if err != nil {
			return err
		}
		if version >= len(migrations) {
			break
		}
		logger.Info("Applying migration", "version", version)
		_, err = db.ExecContext(
			ctx,
			"BEGIN TRANSACTION;\n"+
				migrations[version]+
				"\nCOMMIT;")
		if err != nil {
			return err
		}
		logger.Info("Migration applied", "version", version)
		version++
		_, err = db.Exec("PRAGMA user_version = ?", version)
		if err != nil {
			return err
		}
	}
	return nil
}

func newDB(path string) *db {
	return &db{
		s}
}
