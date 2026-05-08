package main

import (
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/charmbracelet/bubbles/table"
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

func loadGamesCmd(fingerPrint string) tea.Cmd {
	return func() tea.Msg {
		games, err := dataManager.GetGamesForPlayer(fingerPrint)
		return loadGamesMsg{games: games, err: err}
	}
}

func gameRowsFor(fingerPrint string, games []common.Game) []table.Row {
	rows := make([]table.Row, 0, len(games))
	for _, g := range games {
		var color, opponent string
		if g.WhiteFingerprint == fingerPrint {
			color = "white"
			opponent = g.BlackUsername
		} else {
			color = "black"
			opponent = g.WhiteUsername
		}
		if opponent == "" {
			opponent = "?"
		}
		rows = append(rows, table.Row{
			g.CreatedAt.Format("2006-01-02 15:04"),
			color,
			opponent,
			g.Outcome,
			g.Method,
		})
	}
	return rows
}

func startGamesLoad(m model) (model, tea.Cmd) {
	if m.player == nil {
		return m, nil
	}
	m.gamesLoading = true
	m.gamesErr = ""
	return m, tea.Batch(m.usernameSpinner.Tick, loadGamesCmd(m.fingerPrint))
}

func (m model) ViewIntro() string {
	rookArt := `
       .::.
      _|||||_
     | || || |
     |_______|
     \__ ___ /
      |___|_| 
      |_|___| 
      |___|_| 
     (_______)
     /_______\`

	termArt := `  ____
 | >_ |
 |____|`

	textArt := `
  _____ _____ ____  __  __       ____ _   _ _____ ____ ____ 
 |_   _| ____|  _ \|  \/  |     / ___| | | | ____/ ___/ ___|
   | | |  _| | |_) | |\/| |____| |   | |_| |  _| \___ \___ \
   | | | |___|  _ <| |  | |____| |___|  _  | |___ ___) |__) |
   |_| |_____|_| \_\_|  |_|     \____|_| |_|_____|____/____/`

	rookBlock := m.renderer.NewStyle().Foreground(lipgloss.Color("252")).Bold(true).Render(strings.Trim(rookArt, "\n"))
	
	termBlock := strings.Trim(termArt, "\n")
	termStyle := m.renderer.NewStyle().Foreground(lipgloss.Color("42")).Bold(true).MarginTop(4).MarginLeft(2)
	termBlock = termStyle.Render(termBlock)

	topBlock := lipgloss.JoinHorizontal(lipgloss.Top, rookBlock, termBlock)

	textBlock := m.renderer.NewStyle().Foreground(lipgloss.Color("38")).Bold(true).Render(strings.Trim(textArt, "\n"))

	artBlock := lipgloss.JoinVertical(lipgloss.Center, topBlock, "", "", textBlock)
	lines := []string{artBlock, "", ""}

	switch {
	case m.introLoading:
		lines = append(lines, m.usernameSpinner.View()+" loading profile...")
	case m.player == nil:
		lines = append(lines, m.usernameInput.View())
		if m.introErr != "" {
			lines = append(lines, m.renderer.NewStyle().Foreground(lipgloss.Color("9")).Render(m.introErr))
		}
		if m.introSaving {
			lines = append(lines, m.usernameSpinner.View()+" saving profile...")
		}
	default:
		lines = append(lines, "Welcome, "+m.player.Username, "", "Your games:")
		switch {
		case m.gamesLoading:
			lines = append(lines, m.usernameSpinner.View()+" loading games...")
		case m.gamesErr != "":
			lines = append(lines, m.renderer.NewStyle().Foreground(lipgloss.Color("9")).Render(m.gamesErr))
		case len(m.gamesTable.Rows()) == 0:
			lines = append(lines, m.renderer.NewStyle().Faint(true).Render("No games yet."))
		default:
			lines = append(lines, m.gamesTable.View())
		}
	}

	return lipgloss.JoinVertical(lipgloss.Center, lines...)
}

func (m model) UpdateIntro(msg tea.Msg) (model, tea.Cmd) {
	if m.introBusy() {
		var spCmd tea.Cmd
		m.usernameSpinner, spCmd = m.usernameSpinner.Update(msg)

		var tiCmd tea.Cmd
		if _, ok := msg.(tea.KeyMsg); !ok {
			m.usernameInput, tiCmd = m.usernameInput.Update(msg)
		}

		if lm, ok := msg.(loadPlayerMsg); ok {
			m.introLoading = false
			if lm.err != nil {
				m.introErr = lm.err.Error()
				return m, tea.Batch(spCmd, tiCmd, m.usernameInput.Focus())
			}

			m.introErr = ""
			m.player = lm.player
			if lm.player != nil {
				gameManager.SetPlayer(m.fingerPrint, lm.player.Username)
				m.usernameInput.SetValue(lm.player.Username)
				var loadCmd tea.Cmd
				m, loadCmd = startGamesLoad(m)
				return m, tea.Batch(spCmd, tiCmd, loadCmd)
			}
			return m, tea.Batch(spCmd, tiCmd, m.usernameInput.Focus())
		}

		if sm, ok := msg.(savePlayerMsg); ok {
			m.introSaving = false
			if sm.err != nil {
				m.introErr = sm.err.Error()
				return m, tea.Batch(spCmd, tiCmd, m.usernameInput.Focus())
			}
			m.introErr = ""
			m.player = &sm.player
			var loadCmd tea.Cmd
			m, loadCmd = startGamesLoad(m)
			m = m.navigateTo(PageChat)
			return m, tea.Batch(spCmd, tiCmd, loadCmd, m.chatTextarea.Focus())
		}

		return m, tea.Batch(spCmd, tiCmd)
	}

	if m.player != nil {
		var tblCmd tea.Cmd
		m.gamesTable, tblCmd = m.gamesTable.Update(msg)

		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			m = m.navigateTo(PageChat)
			return m, tea.Batch(tblCmd, m.chatTextarea.Focus())
		}
		return m, tblCmd
	}

	var cmd tea.Cmd
	m.usernameInput, cmd = m.usernameInput.Update(msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.introErr != "" && msg.String() != "enter" {
			m.introErr = ""
		}
		switch msg.String() {
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
