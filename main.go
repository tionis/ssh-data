package main

import (
	"context"
	"fmt"
	"github.com/tionis/ssh-data/server"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	app := &cli.App{
		Name:  "ssh-data",
		Usage: "ssh-data",
		Commands: []*cli.Command{
			//{
			//	Name: "ssi-server",
			//	Flags: []cli.Flag{
			//		&cli.StringFlag{
			//			Name:    "db-path",
			//			Aliases: []string{"d"},
			//			Value:   path.Join(home, ".ssh", "data.db"),
			//		},
			//		&cli.StringFlag{
			//			Name:    "log-level",
			//			Aliases: []string{"ll"},
			//			Value:   "info",
			//			Usage:   "log level (debug, info, warn, error)",
			//		},
			//		&cli.StringFlag{
			//			Name:     "server",
			//			Aliases:  []string{"s"},
			//			Required: true,
			//			Usage:    "ssi server to connect to",
			//		},
			//	},
			//	Usage: "start the ssh-data ssi server",
			//	Action: func(c *cli.Context) error {
			//		var logLevel slog.Level
			//		switch strings.ToLower(c.String("log-level")) {
			//		case "debug":
			//			logLevel = slog.LevelDebug
			//		case "info":
			//			logLevel = slog.LevelInfo
			//		case "warn":
			//			logLevel = slog.LevelWarn
			//		case "error":
			//			logLevel = slog.LevelError
			//		case "":
			//			logLevel = slog.LevelWarn
			//		default:
			//			log.Fatalf("invalid log level: %s", c.String("log-level"))
			//		}
			//		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			//			Level: logLevel,
			//		}))
			//		err = startSSIServer(logger, c.String("db-path"), c.String("server"))
			//		if err != nil {
			//			logger.Error("Error starting user server", "error", err)
			//		}
			//		return err
			//	},
			//},
			{
				Name:    "user-server",
				Aliases: []string{"u"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "db-path",
						Aliases: []string{"d"},
						Value:   path.Join(home, ".ssh", "data.db"),
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"ll"},
						Value:   "info",
						Usage:   "log level (debug, info, warn, error)",
					},
				},
				Usage: "start the ssh-data user server (this communicates over stdin/stdout to be called on a normal ssh server)",
				Action: func(c *cli.Context) error {
					var logLevel slog.Level
					switch strings.ToLower(c.String("log-level")) {
					case "debug":
						logLevel = slog.LevelDebug
					case "info":
						logLevel = slog.LevelInfo
					case "warn":
						logLevel = slog.LevelWarn
					case "error":
						logLevel = slog.LevelError
					case "":
						logLevel = slog.LevelWarn
					default:
						log.Fatalf("invalid log level: %s", c.String("log-level"))
					}
					logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
						Level: logLevel,
					}))
					err = startUserServer(logger, c.String("db-path"))
					if err != nil {
						logger.Error("Error starting user server", "error", err)
					}
					return err
				},
			},
			{
				Name:    "server",
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "listen",
						Aliases: []string{"l", "host"},
						Value:   "127.0.0.1",
						Usage:   "host address to listen on for ssh server",
					},
					&cli.StringFlag{
						Name:    "port",
						Aliases: []string{"p"},
						Value:   "22",
						Usage:   "port to listen on for ssh server",
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"ll"},
						Value:   "info",
						Usage:   "log level (debug, info, warn, error)",
					},
					&cli.StringFlag{
						Name:    "data-dir",
						Aliases: []string{"d"},
						Value:   "data",
						Usage:   "directory to store user data",
					},
				},
				Usage: "start the ssh-data server",
				Action: func(c *cli.Context) error {
					var logLevel slog.Level
					switch strings.ToLower(c.String("log-level")) {
					case "debug":
						logLevel = slog.LevelDebug
					case "info":
						logLevel = slog.LevelInfo
					case "warn":
						logLevel = slog.LevelWarn
					case "error":
						logLevel = slog.LevelError
					case "":
						logLevel = slog.LevelInfo
					default:
						log.Fatalf("invalid log level: %s", c.String("log-level"))
					}
					logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
						Level: logLevel,
					}))
					return startServer(logger, c.String("host"), c.String("port"), c.String("data-dir"))
				},
			},
			{
				Name:        "parse_authorized_keys",
				Description: "parse an authorized_keys file and output the data in json format",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "authorized-keys",
						Aliases:  []string{"a"},
						Required: true,
						Usage:    "authorized_keys file to parse",
						Value:    path.Join(home, ".ssh", "authorized_keys"),
					},
				},
				Action: func(c *cli.Context) error {
					authFile, err := os.Open(c.String("authorized-keys"))
					if err != nil {
						return fmt.Errorf("could not open authorized_keys file: %w", err)
					}
					defer func(authFile *os.File) {
						_ = authFile.Close()
					}(authFile)
					authBytes, err := io.ReadAll(authFile)
					if err != nil {
						return fmt.Errorf("could not read authorized_keys file: %w", err)
					}
					ssh.ParseAuthorizedKey(authBytes)
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

//func startSSIServer(logger *slog.Logger, dbPath, server string) error {
//	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
//	defer stop()
//	srv, err := server.NewSSIServer(logger, ctx, dbPath, server)
//	if err != nil {
//		return err
//	}
//	return srv.Start(os.Stdin, os.Stdout)
//}

func startUserServer(logger *slog.Logger, dbPath string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()
	srv, err := server.NewUserServer(logger, ctx, dbPath)
	if err != nil {
		return err
	}
	return srv.Start(os.Stdin, os.Stdout)
}

func startServer(logger *slog.Logger, host, port string, dataDir string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()
	srv := server.New(logger, ctx, host, port, dataDir)
	return srv.Start()
}
