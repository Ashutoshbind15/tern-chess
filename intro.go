package main

import (
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/common"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func loadPlayerCmd(fingerPrint string) tea.Cmd {
	return func() tea.Msg {
		player, err := dataManager.GetPlayer(fingerPrint)
		return loadPlayerMsg{player: player, err: err}
	}
}

func savePlayerCmd(p common.Player, fingerPrint string) tea.Cmd {
	return func() tea.Msg {
		err := dataManager.AddPlayer(p)
		if err == nil {
			gameManager.SetPlayer(fingerPrint, p.Username)
		}
		return savePlayerMsg{player: p, err: err}
	}
}

func (m model) ViewIntro() string {
	lines := []string{"Intro Page"}

	switch {
	case m.introLoading:
		lines = append(lines, m.usernameSpinner.View()+" loading profile...")
	case m.player == nil:
		lines = append(lines, m.usernameInput.View())
		if m.introErr != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.introErr))
		}
		if m.introSaving {
			lines = append(lines, m.usernameSpinner.View()+" saving profile...")
		}
	default:
		lines = append(lines, "Welcome, "+m.player.Username)
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m model) UpdateIntro(msg tea.Msg) (model, tea.Cmd) {
	if m.introBusy() {
		var spCmd tea.Cmd
		m.usernameSpinner, spCmd = m.usernameSpinner.Update(msg)

		if lm, ok := msg.(loadPlayerMsg); ok {
			m.introLoading = false
			if lm.err != nil {
				m.introErr = lm.err.Error()
				m.usernameInput.Focus()
				return m, spCmd
			}

			m.introErr = ""
			m.player = lm.player
			if lm.player != nil {
				gameManager.SetPlayer(m.fingerPrint, lm.player.Username)
				m.usernameInput.SetValue(lm.player.Username)
			}
			return m, spCmd
		}

		if sm, ok := msg.(savePlayerMsg); ok {
			m.introSaving = false
			if sm.err != nil {
				m.introErr = sm.err.Error()
				m.usernameInput.Focus()
				return m, spCmd
			}
			m.introErr = ""
			m.player = &sm.player
			m = m.navigateTo(PageChat)
			return m, spCmd
		}

		return m, spCmd
	}

	var cmd tea.Cmd
	m.usernameInput, cmd = m.usernameInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.introErr != "" && msg.String() != "enter" {
			m.introErr = ""
		}
		switch msg.String() {
		case "esc":
			if m.player != nil {
				m = m.navigateTo(PageChat)
			}
		case "enter":
			username := strings.TrimSpace(m.usernameInput.Value())
			if m.player == nil && username != "" {
				m.usernameInput.SetValue(username)
				p := common.Player{Fingerprint: m.fingerPrint, Username: username}
				m.introErr = ""
				m.introSaving = true
				m.usernameInput.Blur()
				return m, tea.Batch(m.usernameSpinner.Tick, savePlayerCmd(p, m.fingerPrint))
			}
		}
	}

	return m, cmd
}
