package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
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
)

const (
	host = "localhost"
	port = "23234"
)

var sessionManager *SessionManager

func main() {
	
	sessionManager = NewSessionManager()

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
		m:= model{
			counter:   0,
		}
		program := tea.NewProgram(m, append(bubbletea.MakeOptions(s), tea.WithAltScreen())...)

		fingerPrint := s.Context().Value("fingerprint").(string)
		sessionManager.SetProgram(fingerPrint, program)
		
		return program
	}
	return bubbletea.MiddlewareWithProgramHandler(teaHandler, termenv.ANSI256)
}

// Just a generic tea.Model to demo terminal information of ssh.
type model struct {
	counter   int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	m.counter++
	return m, nil
}

func (m model) View() string {
	s := fmt.Sprintf("counter: %d", m.counter)
	return s
}