package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"strconv"
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
                validUntil INTEGER NOT NULL DEFAULT -1, -- -1 means never expires
                lockedUntil INTEGER NOT NULL DEFAULT -1 -- -1 means unlocked
        );
		CREATE TABLE authorized_keys( -- TODO just use json for this?
		    pubKey TEXT PRIMARY KEY,
		    principals JSON, -- store as a json array, as it's a pattern list that needs to be checked in series
		    isCA BOOL NOT NULL DEFAULT FALSE,
		    expiryTime INTEGER NOT NULL DEFAULT -1, -- -1 means never expires
		    fromIP JSON,
			options JSON NOT NULL DEFAULT '{}' -- raw options
		);`,
	}
)

// TODO add custom funcs to handle string manipulation and maybe extend json handling
// TODO implement all the data handling functions

func NewUserDB(ctx context.Context, dbPath string, logger *slog.Logger) (*UserDB, error) {
	db, err := sql.Open("sqlite3", dbPath+"?_fk=true&_timeout=5000&_journal_mode=WAL")
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
		return nil, fmt.Errorf("could not apply migrations: %w", err)
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
		if !query.Next() {
			return fmt.Errorf("could not get user_version")
		}
		err = query.Scan(&version)
		if err != nil {
			return fmt.Errorf("could not get user_version: %w", err)
		}
		err = query.Close()
		if err != nil {
			return fmt.Errorf("could not close query: %w", err)
		}
		if version >= len(migrations) {
			break
		}
		db.logger.Info("Applying migration", "version", version)
		_, err = db.db.ExecContext(db.ctx,
			"BEGIN TRANSACTION;"+
				migrations[version]+
				"PRAGMA user_version = "+strconv.Itoa(version+1)+";"+
				"COMMIT;")
		if err != nil {
			return fmt.Errorf("could not apply migration: %w", err)
		}
		db.logger.Info("Migration applied", "version", version)
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
