package main

import (
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	bullet = "•"
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

func newRendererListStyles(r *lipgloss.Renderer) list.Styles {
	verySubduedColor := lipgloss.AdaptiveColor{Light: "#DDDADA", Dark: "#3C3C3C"}
	subduedColor := lipgloss.AdaptiveColor{Light: "#9B9B9B", Dark: "#5C5C5C"}

	s := list.Styles{}
	s.TitleBar = r.NewStyle().Padding(0, 0, 1, 2)
	s.Title = r.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1)
	s.Spinner = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#8E8E8E", Dark: "#747373"})
	s.FilterPrompt = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#ECFD65"})
	s.FilterCursor = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"})
	s.DefaultFilterCharacterMatch = r.NewStyle().Underline(true)
	s.StatusBar = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
		Padding(0, 0, 1, 2)
	s.StatusEmpty = r.NewStyle().Foreground(subduedColor)
	s.StatusBarActiveFilter = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"})
	s.StatusBarFilterCount = r.NewStyle().Foreground(verySubduedColor)
	s.NoItems = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"})
	s.ArabicPagination = r.NewStyle().Foreground(subduedColor)
	s.PaginationStyle = r.NewStyle().PaddingLeft(2)
	s.HelpStyle = r.NewStyle().Padding(1, 0, 0, 2)
	s.ActivePaginationDot = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#847A85", Dark: "#979797"}).
		SetString(bullet)
	s.InactivePaginationDot = r.NewStyle().
		Foreground(verySubduedColor).
		SetString(bullet)
	s.DividerDot = r.NewStyle().
		Foreground(verySubduedColor).
		SetString(" " + bullet + " ")
	return s
}

func newRendererItemStyles(r *lipgloss.Renderer) list.DefaultItemStyles {
	s := list.DefaultItemStyles{}
	s.NormalTitle = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#dddddd"}).
		Padding(0, 0, 0, 2)
	s.NormalDesc = s.NormalTitle.
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"})
	s.SelectedTitle = r.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}).
		Foreground(lipgloss.AdaptiveColor{Light: "#EE6FF8", Dark: "#EE6FF8"}).
		Padding(0, 0, 0, 1)
	s.SelectedDesc = s.SelectedTitle.
		Foreground(lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"})
	s.DimmedTitle = r.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#A49FA5", Dark: "#777777"}).
		Padding(0, 0, 0, 2)
	s.DimmedDesc = s.DimmedTitle.
		Foreground(lipgloss.AdaptiveColor{Light: "#C2B8C2", Dark: "#4D4D4D"})
	s.FilterMatch = r.NewStyle().Underline(true)
	return s
}

func newPageList(width, height int, r *lipgloss.Renderer) list.Model {
	items := []list.Item{
		pageMenuItem{page: PageIntro, title: "Intro", description: "Set or view your username"},
		pageMenuItem{page: PageChat, title: "Chat", description: "Lobby chat"},
		pageMenuItem{page: PageGame, title: "Game", description: "Play a game"},
	}
	delegate := list.NewDefaultDelegate()
	delegate.Styles = newRendererItemStyles(r)
	delegate.SetSpacing(1)
	l := list.New(items, delegate, width, height)
	l.Styles = newRendererListStyles(r)
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
