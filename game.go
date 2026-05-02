package main

import (
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/managers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func notifyOpponentJoined(gameID string, joinerFingerprint string) {
	opp := gameManager.OpponentFingerprint(gameID, joinerFingerprint)
	if opp == "" {
		return
	}
	if prog := sessionManager.GetProgram(opp); prog != nil {
		prog.Send(opponentJoinedGameMsg{})
	}
}

const (
	gamePageTitle      = "Game Page"
	gameHelpCreate     = "Create a game: ctrl+n"
	gameHelpJoinRandom = "Join a random game: ctrl+r"
	gameHelpJoinByID   = "Join by ID: type the game ID below and press enter"
	gameNoGame         = "No game"
)

func gamePageCommonRows(m model) []string {
	return []string{
		gamePageTitle,
		"",
		gameHelpCreate,
		gameHelpJoinRandom,
		gameHelpJoinByID,
		"",
		m.gameJoinInput.View(),
	}
}

func gameStatusLine(status string) string {
	switch status {
	case managers.GameStatusWaiting:
		return "Status: waiting for an opponent."
	case managers.GameStatusInProgress:
		return "Status: in progress — play when it is your turn."
	case managers.GameStatusFinished:
		return "Status: finished."
	default:
		if status == "" {
			return ""
		}
		return "Status: " + status
	}
}

func (m model) UpdateGame(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, nil
		case "ctrl+n":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			_, err := gameManager.CreateGame(m.fingerPrint)
			if err != nil {
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			return m, nil
		case "ctrl+r":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			gameID, err := gameManager.JoinRandomGame(m.fingerPrint)
			if err != nil {
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			notifyOpponentJoined(gameID, m.fingerPrint)
			return m, nil
		case "enter":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				return m, nil
			}

			_, err := gameManager.JoinGame(m.fingerPrint, gameID)
			if err != nil {
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			notifyOpponentJoined(gameID, m.fingerPrint)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.gameJoinInput, cmd = m.gameJoinInput.Update(msg)
	return m, cmd
}

func (m model) getGameBoard() string {
	if m.currentGame != nil && m.currentGame.Game() != nil {
		return m.currentGame.Game().FEN()
	}
	return gameNoGame
}

func (m model) ViewGame() string {
	rows := gamePageCommonRows(m)

	if m.currentGame == nil {
		rows = append(rows, "", gameNoGame)
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	status := m.currentGame.Status()
	rows = append(rows, m.getGameBoard(), gameStatusLine(status))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
