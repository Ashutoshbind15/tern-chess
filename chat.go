package main

import (
	"fmt"
	"hash/fnv"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// userPalette is a small set of ANSI 256 colors picked to read well on both
// light and dark terminals. usernameColor hashes a name into this palette so
// each user gets a stable, distinct color across the session.
var userPalette = []lipgloss.Color{
	lipgloss.Color("39"),  // sky blue
	lipgloss.Color("209"), // orange
	lipgloss.Color("213"), // pink
	lipgloss.Color("78"),  // green
	lipgloss.Color("220"), // yellow
	lipgloss.Color("105"), // lavender
	lipgloss.Color("173"), // peach
	lipgloss.Color("51"),  // cyan
	lipgloss.Color("198"), // magenta
	lipgloss.Color("156"), // mint
}

func usernameColor(name string) lipgloss.Color {
	if name == "" {
		return lipgloss.Color("245")
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return userPalette[int(h.Sum32())%len(userPalette)]
}

type chatModel struct {
	ctx          *Context
	chatTextarea textarea.Model
	messages     []message
	onlineCount  int
	joined       bool
}

func newChatModel(ctx *Context) chatModel {
	ta := common.InitTextArea()
	applyRendererTextareaStyles(&ta, ctx.renderer)
	return chatModel{ctx: ctx, chatTextarea: ta}
}

func (m chatModel) Init() tea.Cmd { return nil }

// Activate is called every time the user lands on the chat page (including
// returning from the menu). It joins the room on the first activation and
// seeds the local view with whatever recent backlog the room has.
func (m chatModel) Activate() (chatModel, tea.Cmd) {
	if m.ctx.player == nil {
		return m, m.chatTextarea.Focus()
	}
	if !m.joined {
		prog := sessionManager.GetProgram(m.ctx.fingerPrint)
		if prog == nil {
			return m, m.chatTextarea.Focus()
		}
		backlog := chatRoom.Join(m.ctx.fingerPrint, prog, m.ctx.player.Username)
		m.messages = backlog
		m.joined = true
	}
	return m, m.chatTextarea.Focus()
}

// Deactivate is called by the root model when the user navigates away from
// the chat page (but not when temporarily opening the menu). It leaves the
// room so further messages are not delivered, and clears the local buffer.
func (m chatModel) Deactivate() chatModel {
	if !m.joined {
		return m
	}
	chatRoom.Leave(m.ctx.fingerPrint)
	m.joined = false
	m.messages = nil
	m.onlineCount = 0
	return m
}

func (m chatModel) Update(msg tea.Msg) (chatModel, tea.Cmd) {
	var tiCmd tea.Cmd
	m.chatTextarea, tiCmd = m.chatTextarea.Update(msg)
	var rescmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "enter" {
			if m.ctx.player == nil || !m.joined {
				return m, tiCmd
			}
			cmd := chatRoom.Broadcast(m.ctx.fingerPrint, message{
				sender:  m.ctx.player.Username,
				content: m.chatTextarea.Value(),
			})
			if cmd != nil {
				rescmds = append(rescmds, cmd)
			}
			m.chatTextarea.Reset()
		}
	case message:
		m.messages = append(m.messages, msg)
		if len(m.messages) > chatMaxClientLines {
			m.messages = m.messages[len(m.messages)-chatMaxClientLines:]
		}
	case presenceMsg:
		m.onlineCount = msg.count
	}

	rescmds = append(rescmds, tiCmd)
	return m, tea.Batch(rescmds...)
}

func (m chatModel) renderMessage(msg message) string {
	r := m.ctx.renderer

	timeStyle := r.NewStyle().Foreground(lipgloss.Color("240")).Faint(true)
	msgStyle := r.NewStyle().Foreground(lipgloss.Color("252"))
	sysStyle := r.NewStyle().Foreground(lipgloss.Color("244")).Italic(true)

	var ts string
	if !msg.at.IsZero() {
		ts = timeStyle.Render("[" + msg.at.Format("15:04") + "] ")
	}

	if msg.system {
		switch {
		case msg.sender != "" && msg.content == "joined":
			name := r.NewStyle().Foreground(usernameColor(msg.sender)).Bold(true).Render(msg.sender)
			arrow := r.NewStyle().Foreground(lipgloss.Color("78")).Render("→ ")
			return ts + arrow + name + sysStyle.Render(" joined")
		case msg.sender != "" && msg.content == "left":
			name := r.NewStyle().Foreground(usernameColor(msg.sender)).Bold(true).Render(msg.sender)
			arrow := r.NewStyle().Foreground(lipgloss.Color("203")).Render("← ")
			return ts + arrow + name + sysStyle.Render(" left")
		default:
			return ts + sysStyle.Render("• "+msg.content)
		}
	}

	sender := r.NewStyle().Foreground(usernameColor(msg.sender)).Bold(true).Render(msg.sender + ":")
	return ts + sender + " " + msgStyle.Render(msg.content)
}

func (m chatModel) View() string {
	r := m.ctx.renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1).
		MarginBottom(1)

	presenceStyle := r.NewStyle().
		Foreground(lipgloss.Color("245")).
		Faint(true)

	title := titleStyle.Render("Chatroom")
	presence := presenceStyle.Render(fmt.Sprintf("%d online", m.onlineCount))
	rows := []string{lipgloss.JoinHorizontal(lipgloss.Left, title, " ", presence)}

	if len(m.messages) == 0 {
		rows = append(rows, r.NewStyle().Faint(true).Render("It's quiet in here. Say hi!"))
	} else {
		for _, msg := range m.messages {
			rows = append(rows, m.renderMessage(msg))
		}
	}

	rows = append(rows, "", m.chatTextarea.View())
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
