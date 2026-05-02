package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) UpdateGame(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, nil
		case "ctrl+n":
			if m.player == nil {
				m.gameStatus = "Set a username before creating a game."
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			gameID, err := gameManager.CreateGame(m.fingerPrint)
			if err != nil {
				m.gameStatus = "Create failed: " + err.Error()
				return m, nil
			}

			m.currentGameID = gameID
			m.gameStatus = "Created a new game."
			m.gameJoinInput.SetValue("")
			return m, nil
		case "ctrl+r":
			if m.player == nil {
				m.gameStatus = "Set a username before joining a game."
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			gameID, err := gameManager.JoinRandomGame(m.fingerPrint)
			if err != nil {
				m.gameStatus = "Random join failed: " + err.Error()
				return m, nil
			}

			m.currentGameID = gameID
			m.gameStatus = "Joined a random game."
			m.gameJoinInput.SetValue("")
			return m, nil
		case "enter":
			if m.player == nil {
				m.gameStatus = "Set a username before joining a game."
				m = m.navigateTo(PageIntro)
				return m, nil
			}

			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				m.gameStatus = "Enter a game ID to join a specific game."
				return m, nil
			}

			joinedGameID, err := gameManager.JoinGame(m.fingerPrint, gameID)
			if err != nil {
				m.gameStatus = "Join failed: " + err.Error()
				return m, nil
			}

			m.currentGameID = joinedGameID
			m.gameStatus = "Joined game by ID."
			m.gameJoinInput.SetValue("")
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.gameJoinInput, cmd = m.gameJoinInput.Update(msg)
	return m, cmd
}

func (m model) ViewGame() string {
	currentGame := "No active game yet"
	if m.currentGameID != "" {
		currentGame = "Current game: " + m.currentGameID
	}

	status := "Status: waiting for an action"
	if m.gameStatus != "" {
		status = "Status: " + m.gameStatus
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		"Game Page",
		"",
		"Create a game: ctrl+n",
		"Join a random game: ctrl+r",
		"Join by ID: type the game ID below and press enter",
		"",
		m.gameJoinInput.View(),
		"",
		currentGame,
		status,
	)
}
