package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
			cmds := sessionManager.SendMessage(message{sender: m.fingerPrint, content: m.chatTextarea.Value()})
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
	s := fmt.Sprintf("counter: %d", m.counter)

	s += "\n\n"

	var msgsBuilder strings.Builder
	for _, msg := range m.messages {
		fmt.Fprintf(&msgsBuilder, "%s: %s\n", msg.sender, msg.content)
	}

	s += fmt.Sprintf("messages: %s", msgsBuilder.String())
	s += "\n\n"
	s += m.chatTextarea.View()

	return s
}
