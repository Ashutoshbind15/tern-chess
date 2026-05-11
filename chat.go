package main

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// UpdateChat handles chat-specific update logic
func (m model) UpdateChat(msg tea.Msg) (model, tea.Cmd) {
	var tiCmd tea.Cmd
	m.chatTextarea, tiCmd = m.chatTextarea.Update(msg)
	var rescmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.player == nil {
				return m, tiCmd
			}

			cmds := sessionManager.SendMessage(m.fingerPrint, message{
				sender:  m.player.Username,
				content: m.chatTextarea.Value(),
			})
			rescmds = append(rescmds, cmds...)
			m.chatTextarea.Reset()
		}
	case message:
		m.messages = append(m.messages, msg)
	}

	rescmds = append(rescmds, tiCmd)
	return m, tea.Batch(rescmds...)
}

// ViewChat renders the chat view
func (m model) ViewChat() string {
	titleStyle := m.renderer.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1).
		MarginBottom(1)

	senderStyle := m.renderer.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true)

	msgStyle := m.renderer.NewStyle().
		Foreground(lipgloss.Color("252"))

	var rows []string
	rows = append(rows, titleStyle.Render("Lobby Chat"))

	if len(m.messages) == 0 {
		rows = append(rows, m.renderer.NewStyle().Faint(true).Render("No messages yet. Be the first to say hi!"))
	} else {
		for _, msg := range m.messages {
			sender := senderStyle.Render(msg.sender + ":")
			content := msgStyle.Render(msg.content)
			rows = append(rows, sender+" "+content)
		}
	}

	rows = append(rows, "")
	rows = append(rows, m.chatTextarea.View())

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
