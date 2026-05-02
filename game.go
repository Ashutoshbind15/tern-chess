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

			_, err := gameManager.JoinRandomGame(m.fingerPrint)
			if err != nil {
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
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
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.gameJoinInput, cmd = m.gameJoinInput.Update(msg)
	return m, cmd
}

func (m model) ViewGame() string {
	currentGame := "No active game yet"
	if m.currentGame != nil {
		currentGame = "Current game: " + m.currentGame.ID()
	}

	status := "Status: waiting for an action"
	if m.currentGame != nil && m.currentGame.Status() != "" {
		status = "Status: " + m.currentGame.Status()
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
