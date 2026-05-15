package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	"github.com/joho/godotenv"
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
var botGameManager *managers.BotGameManager
var botAPIManager *managers.BotAPIManager
var chatRoom *ChatRoom

func main() {
	host := sshListenHost()

	sessionManager = NewSessionManager()
	dataManager = managers.NewDataManager()
	gameManager = managers.NewGameManager()
	botGameManager = managers.NewBotGameManager()
	botAPIManager = managers.NewBotAPIManager()
	chatRoom = NewChatRoom()

	clockStop := make(chan struct{})
	defer close(clockStop)
	go runClockTicker(clockStop)

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
			sessionCleanupMiddleware(),
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

		m := newAppModel(fingerPrint, renderer, dump)

		program := tea.NewProgram(m, append(bubbletea.MakeOptions(s), tea.WithAltScreen(), tea.WithMouseCellMotion())...)

		sessionManager.SetProgram(fingerPrint, program)

		return program
	}
	return bubbletea.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

// sessionCleanupMiddleware tears down per-session state once the bubbletea
// program has exited. Placed outside customMiddleWare in the middleware
// chain so its deferred cleanup fires after the program returns.
func sessionCleanupMiddleware() wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			fp, _ := s.Context().Value("fingerprint").(string)
			defer func() {
				if fp != "" {
					chatRoom.Leave(fp)
					sessionManager.RemoveProgram(fp)
				}
			}()
			next(s)
		}
	}
}

type message struct {
	sender  string
	content string
	system  bool
	at      time.Time
}

type presenceMsg struct {
	count int
}

type opponentJoinedGameMsg struct {
	opponentName string
	snapshot     *managers.Snapshot
}

type gameUpdatedMsg struct {
	move     string
	snapshot *managers.Snapshot
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
