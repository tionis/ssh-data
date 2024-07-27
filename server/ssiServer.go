package server

import (
	"context"
	"fmt"
	"golang.org/x/crypto/ssh"
	"io"
	"log/slog"
	"net"
)

type SSIServer struct {
	ctx          context.Context
	userDB       *UserDB
	logger       *slog.Logger
	remoteServer string
}

func NewSSIServer(logger *slog.Logger, context context.Context, dbPath string, server string) (*SSIServer, error) {
	userDB, err := NewUserDB(context, dbPath, logger)
	if err != nil {
		logger.Error("Could not open database", "error", err)
		return nil, fmt.Errorf("could not open database: %w", err)
	}
	return &SSIServer{
		userDB:       userDB,
		logger:       logger,
		ctx:          context,
		remoteServer: server,
	}, nil
}

// data model:
// CREATE TABLE data(
//     key TEXT PRIMARY KEY,
//     type TEXT NOT NULL DEFAULT 'string',
//     value TEXT NOT NULL DEFAULT '',
//     validUntil INTEGER NOT NULL DEFAULT -1
// );
// CREATE TABLE webhooks(
//     path TEXT PRIMARY KEY,
//     command JSON NOT NULL
// );
// CREATE TABLE webhook_tokens(
//     token TEXT PRIMARY KEY,
//     path TEXT NOT NULL
// );

func forward(localConn net.Conn, config *ssh.ClientConfig, serverAddrString, remoteAddrString string) error {
	// Setup sshClientConn (type *ssh.ClientConn)
	sshClientConn, err := ssh.Dial("tcp", serverAddrString, config)
	if err != nil {
		return fmt.Errorf("ssh.Dial failed: %w", err)
	}

	// Setup sshConn (type net.Conn)
	sshConn, err := sshClientConn.Dial("tcp", remoteAddrString)

	// Copy localConn.Reader to sshConn.Writer
	go func() {
		_, err = io.Copy(sshConn, localConn)
		//if err != nil {
		//	return fmt.Errorf("io.Copy failed: %w", err)
		//}
	}()

	// Copy sshConn.Reader to localConn.Writer
	go func() {
		_, err = io.Copy(localConn, sshConn)
		//if err != nil {
		//	return fmt.Errorf("io.Copy failed: %w", err)
		//}
	}()
	return nil
}

//func (s *SSIServer) Start() error {
//	// TDO
//	// open two port ssh port forwards (for ssh and http respectively)
//	sshClientConn, err := ssh.Dial("tcp", s.remoteServer, &ssh.ClientConfig{})
//	if err != nil {
//		s.logger.Error("Could not dial remote server", "error", err)
//		return fmt.Errorf("could not dial remote server: %w", err)
//	}
//	httpsshConn, err := sshClientConn.Dial("tcp", "localhost:80")
//	if err != nil {
//		s.logger.Error("Could not dial remote server", "error", err)
//		return fmt.Errorf("could not dial remote server: %w", err)
//	}
//	sshsshConn, err := sshClientConn.Dial("tcp", "localhost:22")
//	if err != nil {
//		s.logger.Error("Could not dial remote server", "error", err)
//		return fmt.Errorf("could not dial remote server: %w", err)
//	}
//	// TDO start ssh server and http server
//}
