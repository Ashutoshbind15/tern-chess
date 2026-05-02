package main

import (
	"github.com/Ashutoshbind15/ssh-chess/common"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) ViewIntro() string {
	// conditionally render the username or the input for taking the username
	if m.player == nil {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			"Intro Page",
			m.usernameInput.View(),
		)
	} else {
		return lipgloss.JoinVertical(
			lipgloss.Center,
			"Intro Page",
			"Welcome, "+m.player.Username,
		)
	}
}

func (m model) UpdateIntro(msg tea.Msg) (model, tea.Cmd) {
	var cmd tea.Cmd
	m.usernameInput, cmd = m.usernameInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m = m.navigateTo(PageChat)
		case "enter":
			// check if model.player is nil
			// and the text input is non-empty
			if m.player == nil && m.usernameInput.Value() != "" {
				m.player = &common.Player{Fingerprint: m.fingerPrint, Username: m.usernameInput.Value()}
				// todo: make the addition async, and show a loading spinner till then
				dataManager.AddPlayer(*m.player)
				gameManager.SetPlayer(m.fingerPrint, m.player.Username)
				m = m.navigateTo(PageChat)
			}
		}
	}

	return m, cmd
}
