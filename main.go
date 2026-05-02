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

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
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
var gameManager *managers.GameManager

func main() {
	sessionManager = NewSessionManager()
	dataManager = managers.NewDataManager()
	gameManager = managers.NewGameManager()

	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		// wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			hash := md5.Sum(key.Marshal())
			fingerPrint := hex.EncodeToString(hash[:])
			ctx.SetValue("fingerprint", fingerPrint)
			return true
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
		m := initModel(fingerPrint)

		program := tea.NewProgram(m, append(bubbletea.MakeOptions(s), tea.WithAltScreen())...)

		sessionManager.SetProgram(fingerPrint, program)

		return program
	}
	return bubbletea.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

type message struct {
	sender  string
	content string
}

type opponentJoinedGameMsg struct{}

type gameUpdatedMsg struct {
	move string
}

type Page string

const (
	PageIntro  Page = "intro"
	PageChat   Page = "chat"
	PageSelect Page = "select"
	PageGame   Page = "game"
)

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	counter       int
	messages      []message
	chatTextarea  textarea.Model
	usernameInput textinput.Model
	gameJoinInput textinput.Model
	moveInput     textinput.Model
	fingerPrint   string
	page          Page
	previousPage  *Page
	player        *common.Player
	pageList      list.Model
	currentGame   *managers.Game
	gameNotice    string
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) navigateTo(page Page) model {
	// todo: add a toast or some sort of feedback for
	// an unexpected action
	if (page == PageChat || page == PageGame) && m.player == nil {
		m.page = PageIntro
		return m
	}

	m.page = page
	return m
}

func (m model) openPageSelect() model {
	if m.page == PageSelect {
		return m
	}

	previousPage := m.page
	m.previousPage = &previousPage
	m.page = PageSelect
	return m
}

func (m model) closePageSelect() model {
	if m.previousPage == nil {
		return m
	}

	m.page = *m.previousPage
	m.previousPage = nil
	return m
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// for updating game state for the opponent
	// program in real-time
	if _, ok := msg.(opponentJoinedGameMsg); ok {
		m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
		if m.currentGame != nil && m.currentGame.Status() == managers.GameStatusInProgress {
			m.gameNotice = "Opponent joined. Game on."
		}
	}
	if msg, ok := msg.(gameUpdatedMsg); ok {
		m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
		if msg.move != "" {
			m.gameNotice = "Opponent played " + msg.move + "."
		}
	}

	// Handle global commands
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m = m.openPageSelect()
		}
	}

	// Route to page-specific update handlers
	switch m.page {
	case PageSelect:
		var cmd tea.Cmd
		m, cmd = m.UpdateSelect(msg)
		m.counter++
		return m, cmd
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
	case PageGame:
		var cmd tea.Cmd
		m, cmd = m.UpdateGame(msg)
		m.counter++
		return m, cmd
	default:
		m.counter++
		return m, nil
	}
}

func (m model) View() string {
	switch m.page {
	case PageSelect:
		return m.ViewSelect()
	case PageChat:
		return m.ViewChat()
	case PageIntro:
		return m.ViewIntro()
	case PageGame:
		return m.ViewGame()
	default:
		return "Unknown page"
	}
}
