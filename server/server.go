package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"
)

type Server struct {
	userDBs   map[string]*UserDB
	userDBMux sync.Mutex
	host      string
	port      string
	logger    *slog.Logger
	dataDir   string
	context   context.Context
}

func New(logger *slog.Logger, context context.Context, host, port string, dataDir string) *Server {
	return &Server{
		userDBs: make(map[string]*UserDB),
		host:    host,
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
	srv, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(s.host, s.port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	s.logger.Info("Starting SSH server", "host", s.host, "port", s.port)
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			s.logger.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	s.logger.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := srv.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		s.logger.Error("Could not stop server", "error", err)
		return fmt.Errorf("could not stop server: %w", err)
	}
	return nil
}

// You can wire any Bubble Tea model up to the middleware with a function that
// handles the incoming ssh.Session. Here we just grab the terminal info and
// pass it to the new model. You can also return tea.ProgramOptions (such as
// tea.WithAltScreen) on a session by session basis.
func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	// This should never fail, as we are using the activeterm middleware.
	pty, _, _ := s.Pty()

	// When running a Bubble Tea app over SSH, you shouldn't use the default
	// lipgloss.NewStyle function.
	// That function will use the color profile from the os.Stdin, which is the
	// server, not the client.
	// We provide a MakeRenderer function in the bubbletea middleware package,
	// so you can easily get the correct renderer for the current session, and
	// use it to create the styles.
	// The recommended way to use these styles is to then pass them down to
	// your Bubble Tea model.
	renderer := bubbletea.MakeRenderer(s)
	txtStyle := renderer.NewStyle().Foreground(lipgloss.Color("10"))
	quitStyle := renderer.NewStyle().Foreground(lipgloss.Color("8"))

	bg := "light"
	if renderer.HasDarkBackground() {
		bg = "dark"
	}

	m := model{
		term:      pty.Term,
		width:     pty.Window.Width,
		height:    pty.Window.Height,
		bg:        bg,
		txtStyle:  txtStyle,
		quitStyle: quitStyle,
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	term      string
	width     int
	height    int
	bg        string
	txtStyle  lipgloss.Style
	quitStyle lipgloss.Style
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("Your term is %s\nYour window size is %dx%d\nBackground: %s", m.term, m.width, m.height, m.bg)
	return m.txtStyle.Render(s) + "\n\n" + m.quitStyle.Render("Press 'q' to quit\n")
}
