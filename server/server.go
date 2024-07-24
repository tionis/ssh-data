package server

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"path"
	"sync"
)

type Server struct {
	userDBs   map[string]*UserDB
	userDBMux sync.Mutex
	port      int
	logger    *slog.Logger
	dataDir   string
	context   context.Context
}

func New(logger *slog.Logger, context context.Context, port int, dataDir string) *Server {
	return &Server{
		userDBs: make(map[string]*UserDB),
		port:    port,
		context: context,
		logger:  logger,
		dataDir: dataDir,
	}
}

func (s *Server) GetUserDB(username string) (*UserDB, error) {
	s.userDBMux.Lock()
	defer s.userDBMux.Unlock()
	db, ok := s.userDBs[username]
	if !ok {
		// create user dir if needed
		userDir := path.Join(s.dataDir, username)
		err := os.MkdirAll(userDir, 0700)
		if err != nil {
			return nil, err
		}
		// open db
		db, err := sql.Open("sqlite3", path.Join(userDir, "data.db"))
		if err != nil {
			return nil, err
		}
		// apply migrations
		err = applyMigrations(db, s.context, s.logger)
		if err != nil {
			return nil, err
		}
		s.userDBs[username] = &UserDB{db: db}
		return s.userDBs[username], nil
	}
	return db, nil
}

func (s *Server) Start() error {
	s.logger.Info("Starting server", "port", s.port)
	// TODO listen on ssh for connections
	return errors.New("not implemented")
}
