package main

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// pageMenuItem is a list item that maps a menu row to a Page (list.DefaultItem).
type pageMenuItem struct {
	page        Page
	title       string
	description string
}

func (i pageMenuItem) FilterValue() string { return i.title }
func (i pageMenuItem) Title() string       { return i.title }
func (i pageMenuItem) Description() string { return i.description }

func newPageList(width, height int) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetSpacing(1)
	items := []list.Item{
		pageMenuItem{page: PageIntro, title: "Intro", description: "Set or view your username"},
		pageMenuItem{page: PageChat, title: "Chat", description: "Lobby chat"},
		pageMenuItem{page: PageGame, title: "Game", description: "Play a game"},
	}
	l := list.New(items, delegate, width, height)
	l.Title = "Pages"
	l.SetShowStatusBar(false)
	l.DisableQuitKeybindings()
	return l
}

func (m model) UpdateSelect(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if it, ok := m.pageList.SelectedItem().(pageMenuItem); ok {
				m = m.navigateTo(it.page)
				m.previousPage = nil
			}
			return m, nil
		case "esc":
			m = m.closePageSelect()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.pageList, cmd = m.pageList.Update(msg)
	return m, cmd
}

func (m model) ViewSelect() string {
	return m.pageList.View()
}
