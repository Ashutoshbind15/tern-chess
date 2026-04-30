package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/Ashutoshbind15/ssh-chess/managers"
)

const (
	host = "localhost"
	port = "23234"
)

var sessionManager *SessionManager
var dataManager *managers.DataManager

func main() {
	sessionManager = NewSessionManager()
	dataManager = managers.NewDataManager()

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		// wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			hash := md5.Sum(key.Marshal())
			fingerPrint := hex.EncodeToString(hash[:])
			ctx.SetValue("fingerprint", fingerPrint)
			return true;
		}),
		wish.WithMiddleware(
			customMiddleWare(),
			activeterm.Middleware(), // Bubble Tea apps usually require a PTY.
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()

	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
	
}

func customMiddleWare() wish.Middleware {
	teaHandler := func(s ssh.Session) *tea.Program {
		// pty, _, active:= s.Pty()
		fingerPrint := s.Context().Value("fingerprint").(string)
		chatTa := common.InitTextArea()
		usernameInputTa := common.InitTextArea()

		player := dataManager.GetPlayer(fingerPrint)

		m:= model{
			counter:   0,
			messages:  []message{},
			fingerPrint: fingerPrint,
			chatTextarea: chatTa,
			usernameInput: usernameInputTa,
			page: PageIntro,
			player: player,
		}
		
		program := tea.NewProgram(m, append(bubbletea.MakeOptions(s), tea.WithAltScreen())...)
		
		sessionManager.SetProgram(fingerPrint, program)
		
		return program
	}
	return bubbletea.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

type message struct {
	sender string
	content string
}

type Page string

const (
	PageIntro Page = "intro"
	PageGame Page = "game"
	PageChat Page = "chat"
)

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	counter   int
	messages  []message
	chatTextarea  textarea.Model
	usernameInput textarea.Model
	fingerPrint string
	page Page
	player *common.Player
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global commands
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	// Route to page-specific update handlers
	switch m.page {
	case PageChat:
		var cmd tea.Cmd
		m, cmd = m.UpdateChat(msg)
		m.counter++
		return m, cmd
	case PageIntro:
		var cmd tea.Cmd
		m, cmd = m.UpdateIntro(msg)
		m.counter++
		return m, cmd
	default:
		m.counter++
		return m, nil
	}
}

func (m model) View() string {
	switch m.page {
	case PageChat:
		return m.ViewChat()
	case PageIntro:
		return m.ViewIntro()
	default:
		return "Unknown page"
	}
}
