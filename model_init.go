package main

import (
	"io"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// textInputViewWidth must be > 0: bubbles v0.21 textinput placeholderView uses
// make([]rune, m.Width+1) and early-returns when Width is 0, so a zero width
// truncates the placeholder to a single character.
const textInputViewWidth = 40

func newGamesTable(r *lipgloss.Renderer) table.Model {
	columns := []table.Column{
		{Title: "Date", Width: 16},
		{Title: "Color", Width: 6},
		{Title: "Opponent", Width: 18},
		{Title: "Outcome", Width: 8},
		{Title: "Method", Width: 18},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(8),
	)

	styles := table.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	styles.Selected = styles.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(styles)

	return t
}

func applyRendererTextareaStyles(ta *textarea.Model, r *lipgloss.Renderer) {
	focused := textarea.Style{
		Base:             r.NewStyle(),
		CursorLine:       r.NewStyle().Background(lipgloss.AdaptiveColor{Light: "255", Dark: "0"}),
		CursorLineNumber: r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "240"}),
		EndOfBuffer:      r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "254", Dark: "0"}),
		LineNumber:       r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "249", Dark: "7"}),
		Placeholder:      r.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:           r.NewStyle().Foreground(lipgloss.Color("7")),
		Text:             r.NewStyle(),
	}
	blurred := textarea.Style{
		Base:             r.NewStyle(),
		CursorLine:       r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "7"}),
		CursorLineNumber: r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "249", Dark: "7"}),
		EndOfBuffer:      r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "254", Dark: "0"}),
		LineNumber:       r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "249", Dark: "7"}),
		Placeholder:      r.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:           r.NewStyle().Foreground(lipgloss.Color("7")),
		Text:             r.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "245", Dark: "7"}),
	}
	ta.FocusedStyle = focused
	ta.BlurredStyle = blurred
	ta.Cursor.Style = r.NewStyle()
	ta.Cursor.TextStyle = r.NewStyle()
}

func applyRendererTextInputStyles(ti *textinput.Model, r *lipgloss.Renderer) {
	ti.PromptStyle = r.NewStyle()
	ti.TextStyle = r.NewStyle()
	ti.PlaceholderStyle = r.NewStyle().Foreground(lipgloss.Color("240"))
	ti.CompletionStyle = r.NewStyle().Foreground(lipgloss.Color("240"))
	ti.Cursor.Style = r.NewStyle()
	ti.Cursor.TextStyle = r.NewStyle()
}

func initModel(fingerPrint string, renderer *lipgloss.Renderer, dump io.Writer) model {
	chatTa := common.InitTextArea()
	applyRendererTextareaStyles(&chatTa, renderer)

	usernameInputTa := common.InitTextInput()
	applyRendererTextInputStyles(&usernameInputTa, renderer)

	gameJoinInput := common.InitTextInput()
	applyRendererTextInputStyles(&gameJoinInput, renderer)

	moveInput := common.InitTextInput()
	applyRendererTextInputStyles(&moveInput, renderer)

	gameJoinInput.Prompt = "game id> "
	gameJoinInput.Placeholder = "abc123"
	gameJoinInput.Width = textInputViewWidth
	moveInput.Prompt = "move> "
	moveInput.Placeholder = "e2e4"
	moveInput.Width = textInputViewWidth

	usernameInputTa.Width = textInputViewWidth

	return model{
		counter:             0,
		messages:            []message{},
		dump:                dump,
		fingerPrint:         fingerPrint,
		chatTextarea:        chatTa,
		usernameInput:       usernameInputTa,
		usernameSpinner:     common.InitSpinner(),
		gameJoinInput:       gameJoinInput,
		moveInput:           moveInput,
		page:                PageIntro,
		introLoading:        true,
		pageList:            newPageList(80, 22, renderer),
		currentGame:         gameManager.GameForPlayer(fingerPrint),
		gamesTable:          newGamesTable(renderer),
		renderer:            renderer,
		selectedTimeControl: NoTimeControl,
		whiteTimeLeft:       0,
		blackTimeLeft:       0,
		zone:                zone.New(),
	}
}
