package main

import (
	"context"
	"github.com/tionis/ssh-data/server"
	"github.com/urfave/cli/v2"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	app := &cli.App{
		Name:  "ssh-data",
		Usage: "ssh-data",
		Commands: []*cli.Command{
			{
				Name:    "server",
				Aliases: []string{"s"},
				Flags: []cli.Flag{
					&cli.IntFlag{
						Name:    "port",
						Aliases: []string{"p"},
						Value:   22,
						Usage:   "port to listen on for ssh server",
					},
					&cli.StringFlag{
						Name:    "log-level",
						Aliases: []string{"l"},
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
					return startServer(logger, c.Int("port"), c.String("data-dir"))
				},
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func startServer(logger *slog.Logger, port int, dataDir string) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT)
	defer stop()
	srv := server.New(logger, ctx, port, dataDir)
	return srv.Start()
}
