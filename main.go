package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/muesli/termenv"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/Ashutoshbind15/ssh-chess/managers"
	"github.com/davecgh/go-spew/spew"
	"github.com/joho/godotenv"
	zone "github.com/lrstanley/bubblezone"
	"github.com/notnil/chess"
)

const port = "23234"

func init() {
	_ = godotenv.Load()
}

func sshListenHost() string {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("TERN_CHESS_ENV"))) {
	case "prod", "production":
		return "0.0.0.0"
	default:
		return "127.0.0.1"
	}
}

var sessionManager *SessionManager
var dataManager *managers.DataManager
var gameManager *managers.GameManager
var clockManager *managers.ClockManager
var botGameManager *managers.BotGameManager
var botAPIManager *managers.BotAPIManager

func main() {
	host := sshListenHost()

	sessionManager = NewSessionManager()
	dataManager = managers.NewDataManager()
	gameManager = managers.NewGameManager()
	clockManager = managers.NewClockManager(gameManager, dataManager, sessionManager)
	botGameManager = managers.NewBotGameManager()
	botAPIManager = managers.NewBotAPIManager()
	go clockManager.Start()

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
		_, _, active := s.Pty()
		if !active {
			return nil
		}

		renderer := bubbletea.MakeRenderer(s)
		fingerPrint := s.Context().Value("fingerprint").(string)

		var dump *os.File
		if _, ok := os.LookupEnv("DEBUG"); ok {
			var err error
			dump, err = os.OpenFile("messages.log", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				log.Error("DEBUG: could not open messages.log", "error", err)
			}
		}

		m := initModel(fingerPrint, renderer, dump)

		program := tea.NewProgram(m, append(bubbletea.MakeOptions(s), tea.WithAltScreen(), tea.WithMouseCellMotion())...)

		sessionManager.SetProgram(fingerPrint, program)

		return program
	}
	return bubbletea.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

type message struct {
	sender  string
	content string
}

type opponentJoinedGameMsg struct {
	opponentName string
}

type gameUpdatedMsg struct {
	move string
}

type loadPlayerMsg struct {
	player *common.Player
	err    error
}

type savePlayerMsg struct {
	player common.Player
	err    error
}

type loadGamesMsg struct {
	games []common.Game
	err   error
}

type gamesRefreshMsg struct{}

type Page string

const (
	PageIntro  Page = "intro"
	PageChat   Page = "chat"
	PageSelect Page = "select"
	PageGame   Page = "game"
	PageBot    Page = "bot"
)

type TimeControlChoice int

const (
	NoTimeControl TimeControlChoice = 0
	TimeControl1  TimeControlChoice = 1
	TimeControl3  TimeControlChoice = 3
	TimeControl5  TimeControlChoice = 5
)

func (tc TimeControlChoice) ToGameTimeControl() managers.TimeControl {
	return managers.TimeControl(tc)
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	width, height       int
	counter             int
	messages            []message
	chatTextarea        textarea.Model
	usernameInput       textinput.Model
	gameJoinInput       textinput.Model
	moveInput           textinput.Model
	fingerPrint         string
	page                Page
	previousPage        *Page
	player              *common.Player
	pageList            list.Model
	currentGame         *managers.Game
	gameNotice          string
	introLoading        bool
	introSaving         bool
	introErr            string
	usernameSpinner     spinner.Model
	gamesTable          table.Model
	gamesLoading        bool
	gamesErr            string
	renderer            *lipgloss.Renderer
	selectedTimeControl TimeControlChoice
	whiteTimeLeft       time.Duration
	blackTimeLeft       time.Duration
	zone                *zone.Manager
	selected            string
	possibleMoves       []string
	currentBotGame      *managers.BotGame
	botGamesTable       table.Model
	botGamesLoading     bool
	botGamesErr         string
	botSelectedLevel    int
	botSelectedColor    chess.Color
	botNotice           string
	botMoving           bool
	dump                io.Writer
}

func (m model) Init() tea.Cmd {
	if m.introLoading {
		return tea.Batch(
			m.usernameSpinner.Tick,
			loadPlayerCmd(m.fingerPrint),
			m.usernameInput.Focus(),
		)
	}
	return nil
}

func (m model) introBusy() bool {
	return m.introLoading || m.introSaving
}

func (m model) navigateTo(page Page) model {
	// todo: add a toast or some sort of feedback for
	// an unexpected action
	if (page == PageChat || page == PageGame || page == PageBot) && m.player == nil {
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
	if m.dump != nil {
		spew.Fdump(m.dump, msg)
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	}

	// Handle global commands
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.zone != nil {
				m.zone.Close()
			}
			return m, tea.Quit
		case "tab":
			if !m.introBusy() {
				m = m.openPageSelect()
			}
		}
	case gamesRefreshMsg:
		if m.player != nil {
			m.gamesLoading = true
			m.gamesErr = ""
			return m, loadGamesCmd(m.fingerPrint)
		}
		return m, nil
	case loadGamesMsg:
		m.gamesLoading = false
		if msg.err != nil {
			m.gamesErr = msg.err.Error()
			return m, nil
		}
		m.gamesErr = ""
		m.gamesTable.SetRows(gameRowsFor(m.fingerPrint, msg.games))
		return m, nil
	case managers.ClockUpdateMsg:
		if m.currentGame != nil && m.currentGame.ID() == msg.GameID {
			m.whiteTimeLeft = msg.WhiteTime
			m.blackTimeLeft = msg.BlackTime
		}
		return m, nil
	case managers.TimeForfeitMsg:
		if m.currentGame == nil || m.currentGame.ID() != msg.GameID {
			return m, nil
		}
		if msg.LoserColor == chess.White {
			m.gameNotice = "White ran out of time. Black wins!"
		} else {
			m.gameNotice = "Black ran out of time. White wins!"
		}
		m.gamesLoading = true
		return m, loadGamesCmd(m.fingerPrint)
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
	case PageBot:
		var cmd tea.Cmd
		m, cmd = m.UpdateBot(msg)
		m.counter++
		return m, cmd
	default:
		m.counter++
		return m, nil
	}
}

func (m model) headerText() string {
	if m.player != nil {
		return m.player.Username
	}
	return "Guest"
}

func (m model) View() string {
	headerStyle := m.renderer.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Border(lipgloss.NormalBorder(), false, false, true, false).
		BorderForeground(lipgloss.Color("62")).
		Foreground(lipgloss.Color("229")).
		Bold(true)

	header := headerStyle.Render("♜ >_ Term Chess | " + m.headerText())

	footerStyle := m.renderer.NewStyle().
		Align(lipgloss.Center).
		Width(m.width).
		Foreground(lipgloss.Color("241"))

	footer := footerStyle.Render("Page: " + string(m.page) + " | Press tab to open menu")

	var pageContent string
	switch m.page {
	case PageSelect:
		pageContent = m.ViewSelect()
	case PageChat:
		pageContent = m.ViewChat()
	case PageIntro:
		pageContent = m.ViewIntro()
	case PageGame:
		pageContent = m.ViewGame()
	case PageBot:
		pageContent = m.ViewBot()
	default:
		pageContent = "Unknown page"
	}

	content := m.renderer.NewStyle().
		Width(m.width).
		Height(m.height - lipgloss.Height(header) - lipgloss.Height(footer)).
		Render(pageContent)

	output := lipgloss.JoinVertical(lipgloss.Top, header, content, footer)
	if m.zone != nil {
		return m.zone.Scan(output)
	}
	return output
}
