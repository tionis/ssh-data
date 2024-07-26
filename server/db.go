package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"sync"
)

type UserDB struct {
	db          *sql.DB
	channels    map[string]chan string
	channelsMux sync.Mutex
	// TODO support both pubsub and mpmc channels
	ctx    context.Context
	logger *slog.Logger
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

func NewUserDB(ctx context.Context, dbPath string, logger *slog.Logger) (*UserDB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	userDB := &UserDB{
		db:       db,
		ctx:      ctx,
		logger:   logger,
		channels: make(map[string]chan string),
	}
	err = userDB.applyMigrations()
	if err != nil {
		return nil, err
	}
	return userDB, nil
}

func (db *UserDB) applyMigrations() error {
	for {
		query, err := db.db.Query("PRAGMA user_version;")
		if err != nil {
			return fmt.Errorf("could not get user_version: %w", err)
		}
		var version int
		hasValue := query.Next()
		if !hasValue {
			return fmt.Errorf("could not get user_version")
		}
		err = query.Scan(&version)
		if err != nil {
			return fmt.Errorf("could not get user_version: %w", err)
		}
		if version >= len(migrations) {
			break
		}
		db.logger.Info("Applying migration", "version", version)
		_, err = db.db.ExecContext(db.ctx, "BEGIN TRANSACTION;"+migrations[version]+"COMMIT;")
		if err != nil {
			return fmt.Errorf("could not apply migration: %w", err)
		}
		db.logger.Info("Migration applied", "version", version)
		version++
		_, err = db.db.Exec("PRAGMA user_version = ?", version)
		if err != nil {
			return fmt.Errorf("could not set user_version: %w", err)
		}
	}
	return nil
}

// queryToJSON executes a SQL query and returns the result as a JSON string.
func (db *UserDB) queryToJSON(query string, args ...any) (string, error) {
	rows, err := db.db.Query(query, args...)
	if err != nil {
		return "", fmt.Errorf("error executing query %v: %w", query, err)
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return "", fmt.Errorf("error getting column types: %w", err)
	}

	count := len(columnTypes)
	var finalRows []interface{}

	for rows.Next() {
		scanArgs := make([]interface{}, count)
		for i, v := range columnTypes {
			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID", "TIMESTAMP":
				scanArgs[i] = new(sql.NullString)
				break
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
				break
			case "INT4":
				scanArgs[i] = new(sql.NullInt64)
				break
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)
		if err != nil {
			return "", fmt.Errorf("error scanning row: %w", err)
		}

		masterData := map[string]interface{}{}
		for i, v := range columnTypes {

			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				masterData[v.Name()] = z.Bool
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				masterData[v.Name()] = z.String
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				masterData[v.Name()] = z.Int64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				masterData[v.Name()] = z.Float64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				masterData[v.Name()] = z.Int32
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	z, err := json.MarshalIndent(finalRows, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error marshalling json: %w", err)
	}
	return string(z), nil
}

func (db *UserDB) Close() error {
	return db.db.Close()
}

func (db *UserDB) GetChannel(channel string) chan string {
	db.channelsMux.Lock()
	defer db.channelsMux.Unlock()
	ch, ok := db.channels[channel]
	if !ok {
		ch = make(chan string)
		db.channels[channel] = ch
	}
	return ch
}
