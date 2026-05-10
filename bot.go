package main

import (
	"strconv"
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/Ashutoshbind15/ssh-chess/managers"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/notnil/chess"
)

const (
	botPageTitle  = "Play vs Computer"
	botHelpLevels = "Select level: [1] 1100  [3] 1300  [5] 1500  [7] 1700  [9] 1900"
	botHelpColors = "Select color: [w] white  [b] black  [r] random"
	botHelpStart  = "Start a game: ctrl+n"
	botHelpMove   = "Make a move: click a piece, then click its destination square."
	botHelpResign = "Resign: ctrl+x"
)

type botMoveMsg struct {
	gameID string
	move   string
	err    error
}

type loadBotGamesMsg struct {
	games []common.BotGame
	err   error
}

type botGamesRefreshMsg struct{}

func loadBotGamesCmd(fingerprint string) tea.Cmd {
	return func() tea.Msg {
		games, err := dataManager.GetBotGamesForPlayer(fingerprint)
		return loadBotGamesMsg{games: games, err: err}
	}
}

func requestBotMoveCmd(gameID, fen string, level int) tea.Cmd {
	return func() tea.Msg {
		move, err := botAPIManager.BestMove(fen, level)
		return botMoveMsg{gameID: gameID, move: move, err: err}
	}
}

func botGameRowsFor(games []common.BotGame) []table.Row {
	rows := make([]table.Row, 0, len(games))
	for _, g := range games {
		color := g.PlayerColor
		if color == "" {
			color = "?"
		}
		rows = append(rows, table.Row{
			g.CreatedAt.Format("2006-01-02 15:04"),
			color,
			strconv.Itoa(g.BotLevel),
			g.Outcome,
			g.Method,
		})
	}
	return rows
}

func newBotGamesTable() table.Model {
	columns := []table.Column{
		{Title: "Date", Width: 16},
		{Title: "You", Width: 6},
		{Title: "Level", Width: 6},
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

func startBotGamesLoad(m model) (model, tea.Cmd) {
	if m.player == nil {
		return m, nil
	}
	m.botGamesLoading = true
	m.botGamesErr = ""
	return m, loadBotGamesCmd(m.fingerPrint)
}

func persistAndRemoveBotGame(gameID string) {
	record, ok := botGameManager.BuildBotGameRecord(gameID)
	if !ok {
		return
	}
	if err := dataManager.AddBotGame(record); err != nil {
		log.Error("failed to persist bot game", "id", gameID, "error", err)
		return
	}
	botGameManager.RemoveBotGame(gameID)
}

// UpdateBot is the entry point for the bot page. Mirrors the structure of
// UpdateGame but with no clocks and no opponent messaging.
func (m model) UpdateBot(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case botGamesRefreshMsg:
		return startBotGamesLoad(m)
	case loadBotGamesMsg:
		m.botGamesLoading = false
		if msg.err != nil {
			m.botGamesErr = msg.err.Error()
			return m, nil
		}
		m.botGamesErr = ""
		m.botGamesTable.SetRows(botGameRowsFor(msg.games))
		return m, nil
	case botMoveMsg:
		return m.handleBotMove(msg)
	}

	if m.currentBotGame == nil {
		return m.updateBotLobby(msg)
	}
	switch m.currentBotGame.Status() {
	case managers.GameStatusInProgress:
		return m.updateBotInProgress(msg)
	case managers.GameStatusFinished:
		return m.updateBotFinished(msg)
	}
	return m, nil
}

func (m model) updateBotLobby(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, m.chatTextarea.Focus()
		case "1":
			m.botSelectedLevel = 1100
			return m, nil
		case "3":
			m.botSelectedLevel = 1300
			return m, nil
		case "5":
			m.botSelectedLevel = 1500
			return m, nil
		case "7":
			m.botSelectedLevel = 1700
			return m, nil
		case "9":
			m.botSelectedLevel = 1900
			return m, nil
		case "w":
			m.botSelectedColor = chess.White
			return m, nil
		case "b":
			m.botSelectedColor = chess.Black
			return m, nil
		case "r":
			m.botSelectedColor = chess.NoColor
			return m, nil
		case "ctrl+n":
			if m.botSelectedLevel == 0 {
				m.botNotice = "Pick a level first (1/3/5/7/9)."
				return m, nil
			}
			username := ""
			if m.player != nil {
				username = m.player.Username
			}
			game, err := botGameManager.CreateBotGame(m.fingerPrint, username, m.botSelectedColor, m.botSelectedLevel)
			if err != nil {
				m.botNotice = err.Error()
				return m, nil
			}
			m.currentBotGame = game
			m.botNotice = "Game on. You are " + strings.ToLower(game.PlayerColor().Name()) + " vs level " + strconv.Itoa(game.BotLevel()) + "."
			m.selected = ""
			m.possibleMoves = nil

			if !game.IsPlayersTurn() {
				m.botMoving = true
				return m, requestBotMoveCmd(game.ID(), game.FEN(), game.BotLevel())
			}
			return m, nil
		}
	}

	var tblCmd tea.Cmd
	m.botGamesTable, tblCmd = m.botGamesTable.Update(msg)
	return m, tblCmd
}

// updateBotInProgress only handles mouse input plus a couple of keyboard
// shortcuts. Bot games are intentionally mouse-only — there is no UCI text
// input on this page, which keeps the layout simple and avoids interfering
// with bubblezone's column tracking.
func (m model) updateBotInProgress(msg tea.Msg) (model, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		return m.handleBotBoardMouse(mouseMsg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.selected = ""
			m.possibleMoves = nil
			m = m.navigateTo(PageChat)
			return m, m.chatTextarea.Focus()
		case "ctrl+x":
			game, err := botGameManager.Resign(m.fingerPrint)
			if err != nil {
				m.botNotice = err.Error()
				return m, nil
			}
			m.currentBotGame = game
			m.botNotice = "You resigned."
			persistAndRemoveBotGame(game.ID())
			return m, loadBotGamesCmd(m.fingerPrint)
		}
	}
	return m, nil
}

func (m model) updateBotFinished(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.currentBotGame = nil
		m.selected = ""
		m.possibleMoves = nil
		m.botNotice = ""
		return startBotGamesLoad(m)
	}
	return m, nil
}

func (m model) handleBotMove(msg botMoveMsg) (model, tea.Cmd) {
	if m.currentBotGame == nil || m.currentBotGame.ID() != msg.gameID {
		return m, nil
	}
	m.botMoving = false
	if msg.err != nil {
		m.botNotice = "Bot error: " + msg.err.Error()
		return m, nil
	}
	game, err := botGameManager.ApplyBotMove(msg.gameID, msg.move)
	if err != nil {
		m.botNotice = "Bot error: " + err.Error()
		return m, nil
	}
	m.currentBotGame = game
	m.botNotice = "Bot played " + msg.move + "."

	if game.Status() == managers.GameStatusFinished {
		gameID := game.ID()
		persistAndRemoveBotGame(gameID)
		return m, loadBotGamesCmd(m.fingerPrint)
	}
	return m, nil
}

func (m model) handleBotBoardMouse(msg tea.MouseMsg) (model, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if m.botMoving {
		return m, nil
	}
	if !m.currentBotGame.IsPlayersTurn() {
		m.selected = ""
		m.possibleMoves = nil
		return m, nil
	}

	colorIsWhite := m.currentBotGame.PlayerColor() == chess.White
	doesClick := false

	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			pos := convertToChessboardPosition(j, i, colorIsWhite)
			if m.zone.Get(pos).InBounds(msg) {
				doesClick = true

				if m.selected == "" {
					m.selected = pos
					fenOpt, err := chess.FEN(m.currentBotGame.Game().FEN())
					if err != nil {
						return m, nil
					}
					clientChessClient := chess.NewGame(chess.UseNotation(chess.UCINotation{}), fenOpt)
					possibleMoves := clientChessClient.ValidMoves()

					validMovesFromSelected := []string{}
					for _, mv := range possibleMoves {
						moveStr := mv.String()
						moveStrStart := moveStr[:2]
						moveStrEnd := moveStr[2:]
						if moveStrStart == m.selected {
							if len(moveStrEnd) == 3 {
								validMovesFromSelected = append(validMovesFromSelected, moveStrEnd[:2])
							} else {
								validMovesFromSelected = append(validMovesFromSelected, moveStrEnd)
							}
						}
					}
					m.possibleMoves = validMovesFromSelected
				} else {
					moveUCI := m.selected + pos
					game, err := botGameManager.MakePlayerMove(m.fingerPrint, moveUCI)
					if err != nil {
						game2, perr := botGameManager.MakePlayerMove(m.fingerPrint, moveUCI+"q")
						if perr != nil {
							m.botNotice = "Move rejected: " + err.Error()
						} else {
							game = game2
							moveUCI += "q"
						}
					}

					m.selected = ""
					m.possibleMoves = nil

					if game == nil {
						return m, nil
					}

					m.currentBotGame = game
					m.botNotice = "You played " + moveUCI + "."

					if game.Status() == managers.GameStatusFinished {
						gameID := game.ID()
						persistAndRemoveBotGame(gameID)
						return m, loadBotGamesCmd(m.fingerPrint)
					}

					m.botMoving = true
					return m, requestBotMoveCmd(game.ID(), game.FEN(), game.BotLevel())
				}
			}
		}
	}

	if !doesClick {
		m.selected = ""
		m.possibleMoves = nil
	}

	return m, nil
}

func (m model) renderBotBoardFromFEN() string {
	fen := m.currentBotGame.Game().FEN()
	colorIsWhite := m.currentBotGame.PlayerColor() != chess.Black
	return m.renderChessBoard(fen, colorIsWhite)
}

func botStatusLine(g *managers.BotGame) string {
	if g == nil {
		return ""
	}
	switch g.Status() {
	case managers.GameStatusInProgress:
		return "Status: in progress."
	case managers.GameStatusFinished:
		return "Status: finished."
	}
	return ""
}

func botTurnLine(g *managers.BotGame, botMoving bool) string {
	if g == nil {
		return ""
	}
	you := strings.ToLower(g.PlayerColor().Name())
	if g.Status() != managers.GameStatusInProgress {
		return "You are " + you + "."
	}
	if g.IsPlayersTurn() {
		return "You are " + you + ". Your turn."
	}
	if botMoving {
		return "You are " + you + ". Bot is thinking..."
	}
	return "You are " + you + ". Bot to move."
}

func (m model) ViewBot() string {
	if m.currentBotGame == nil {
		return m.viewBotLobby()
	}
	switch m.currentBotGame.Status() {
	case managers.GameStatusInProgress:
		return m.viewBotInProgress()
	case managers.GameStatusFinished:
		return m.viewBotFinished()
	}
	return ""
}

func (m model) viewBotLobby() string {
	rows := []string{
		botPageTitle,
		"",
	}
	if m.botSelectedLevel == 0 {
		rows = append(rows, botHelpLevels)
	} else {
		rows = append(rows, "Level: "+strconv.Itoa(m.botSelectedLevel)+"  (press 1/3/5/7/9 to change)")
	}
	colorChoice := "random"
	switch m.botSelectedColor {
	case chess.White:
		colorChoice = "white"
	case chess.Black:
		colorChoice = "black"
	}
	rows = append(rows, "Color: "+colorChoice+"  ("+botHelpColors+")")
	rows = append(rows, "", botHelpStart)

	if m.botNotice != "" {
		rows = append(rows, "", m.botNotice)
	}
	rows = append(rows, "", "Your bot games:")
	switch {
	case m.botGamesLoading:
		rows = append(rows, m.usernameSpinner.View()+" loading...")
	case m.botGamesErr != "":
		rows = append(rows, m.renderer.NewStyle().Foreground(lipgloss.Color("9")).Render(m.botGamesErr))
	case len(m.botGamesTable.Rows()) == 0:
		rows = append(rows, m.renderer.NewStyle().Faint(true).Render("No bot games yet."))
	default:
		rows = append(rows, m.botGamesTable.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) botHeaderRows() []string {
	rows := []string{
		botPageTitle,
		"",
		"Game ID: " + m.currentBotGame.ID(),
		"Level: " + strconv.Itoa(m.currentBotGame.BotLevel()),
		botStatusLine(m.currentBotGame),
	}
	if turn := botTurnLine(m.currentBotGame, m.botMoving); turn != "" {
		rows = append(rows, turn)
	}
	if m.botNotice != "" {
		rows = append(rows, m.botNotice)
	}
	return rows
}

func (m model) viewBotInProgress() string {
	rows := append(m.botHeaderRows(), "", m.renderBotBoardFromFEN(), "", botHelpMove, botHelpResign)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) viewBotFinished() string {
	rows := append(m.botHeaderRows(), "", m.renderBotBoardFromFEN(), "", "Press esc to return to bot lobby.")
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
