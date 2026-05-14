package main

import (
	"strconv"
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/Ashutoshbind15/ssh-chess/managers"
	"github.com/charmbracelet/bubbles/spinner"
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

type botModel struct {
	ctx *Context

	currentBotGame   *managers.BotGame
	botGamesTable    table.Model
	botSelectedLevel int
	botSelectedColor chess.Color
	botNotice        string
	botMoving        bool
	botSpinner       spinner.Model

	selected      string
	possibleMoves []string

	botGamesLoading bool
	botGamesErr     string
}

func newBotModel(ctx *Context) botModel {
	return botModel{
		ctx:            ctx,
		currentBotGame: botGameManager.GameForPlayer(ctx.fingerPrint),
		botGamesTable:  newBotGamesTable(),
		botSpinner:     common.InitSpinner(),
	}
}

func (m botModel) Init() tea.Cmd { return nil }

func (m botModel) Activate() (botModel, tea.Cmd) {
	if m.currentBotGame == nil {
		return m, func() tea.Msg { return botGamesRefreshMsg{} }
	}
	return m, nil
}

func navigateToChatCmdBot() tea.Cmd {
	return func() tea.Msg { return navigateMsg{page: PageChat} }
}

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

func (m botModel) startBotGamesLoad() (botModel, tea.Cmd) {
	if m.ctx.player == nil {
		return m, nil
	}
	m.botGamesLoading = true
	m.botGamesErr = ""
	return m, tea.Batch(m.botSpinner.Tick, loadBotGamesCmd(m.ctx.fingerPrint))
}

// persistAndReloadBotGameCmd persists a finished bot game and then reloads
// the player's bot games list. Both DB operations run inside the goroutine
// so Update() returns immediately and the event loop stays responsive. The
// reload is chained after the persist (rather than fired as a separate cmd
// in parallel) so the new game is guaranteed to be visible in the result.
func persistAndReloadBotGameCmd(gameID, fingerprint string) tea.Cmd {
	return func() tea.Msg {
		if record, ok := botGameManager.BuildBotGameRecord(gameID); ok {
			if err := dataManager.AddBotGame(record); err != nil {
				log.Error("failed to persist bot game", "id", gameID, "error", err)
			} else {
				botGameManager.RemoveBotGame(gameID)
			}
		}
		games, err := dataManager.GetBotGamesForPlayer(fingerprint)
		return loadBotGamesMsg{games: games, err: err}
	}
}

// Update is the entry point for the bot page. Mirrors the structure of
// UpdateGame but with no clocks and no opponent messaging.
func (m botModel) Update(msg tea.Msg) (botModel, tea.Cmd) {
	switch msg := msg.(type) {
	case botGamesRefreshMsg:
		return m.startBotGamesLoad()
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
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.botSpinner, cmd = m.botSpinner.Update(msg)
		return m, cmd
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

func (m botModel) updateBotLobby(msg tea.Msg) (botModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, navigateToChatCmdBot()
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
			if m.ctx.player != nil {
				username = m.ctx.player.Username
			}
			game, err := botGameManager.CreateBotGame(m.ctx.fingerPrint, username, m.botSelectedColor, m.botSelectedLevel)
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
func (m botModel) updateBotInProgress(msg tea.Msg) (botModel, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		return m.handleBotBoardMouse(mouseMsg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.selected = ""
			m.possibleMoves = nil
			return m, navigateToChatCmdBot()
		case "ctrl+x":
			game, err := botGameManager.Resign(m.ctx.fingerPrint)
			if err != nil {
				m.botNotice = err.Error()
				return m, nil
			}
			m.currentBotGame = game
			m.botNotice = "You resigned."
			return m, persistAndReloadBotGameCmd(game.ID(), m.ctx.fingerPrint)
		}
	}
	return m, nil
}

func (m botModel) updateBotFinished(msg tea.Msg) (botModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m.currentBotGame = nil
		m.selected = ""
		m.possibleMoves = nil
		m.botNotice = ""
		return m.startBotGamesLoad()
	}
	return m, nil
}

func (m botModel) handleBotMove(msg botMoveMsg) (botModel, tea.Cmd) {
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
		return m, persistAndReloadBotGameCmd(game.ID(), m.ctx.fingerPrint)
	}
	return m, nil
}

func (m botModel) handleBotBoardMouse(msg tea.MouseMsg) (botModel, tea.Cmd) {
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
			if m.ctx.zone.Get(pos).InBounds(msg) {
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
					game, err := botGameManager.MakePlayerMove(m.ctx.fingerPrint, moveUCI)
					if err != nil {
						game2, perr := botGameManager.MakePlayerMove(m.ctx.fingerPrint, moveUCI+"q")
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
						return m, persistAndReloadBotGameCmd(game.ID(), m.ctx.fingerPrint)
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

func (m botModel) renderBotBoardFromFEN() string {
	fen := m.currentBotGame.Game().FEN()
	colorIsWhite := m.currentBotGame.PlayerColor() != chess.Black
	return renderChessBoard(m.ctx.renderer, m.ctx.zone, fen, colorIsWhite, m.selected, m.possibleMoves)
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

func (m botModel) View() string {
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

func (m botModel) viewBotLobby() string {
	r := m.ctx.renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1)

	helpStyle := r.NewStyle().Foreground(lipgloss.Color("241"))
	highlightStyle := r.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	infoStyle := r.NewStyle().Foreground(lipgloss.Color("252"))
	noticeStyle := r.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)

	rows := []string{
		titleStyle.Render(botPageTitle),
		"",
	}
	if m.botSelectedLevel == 0 {
		rows = append(rows, helpStyle.Render(botHelpLevels))
	} else {
		rows = append(rows, infoStyle.Render("Level: ")+highlightStyle.Render(strconv.Itoa(m.botSelectedLevel))+helpStyle.Render("  (press 1/3/5/7/9 to change)"))
	}
	colorChoice := "random"
	switch m.botSelectedColor {
	case chess.White:
		colorChoice = "white"
	case chess.Black:
		colorChoice = "black"
	}
	rows = append(rows, infoStyle.Render("Color: ")+highlightStyle.Render(colorChoice)+helpStyle.Render("  ("+botHelpColors+")"))
	rows = append(rows, "", helpStyle.Render(botHelpStart))

	if m.botNotice != "" {
		rows = append(rows, "", noticeStyle.Render(m.botNotice))
	}
	rows = append(rows, "", infoStyle.Render("Your bot games:"))
	switch {
	case m.botGamesLoading:
		rows = append(rows, m.botSpinner.View()+" loading...")
	case m.botGamesErr != "":
		rows = append(rows, r.NewStyle().Foreground(lipgloss.Color("9")).Render(m.botGamesErr))
	case len(m.botGamesTable.Rows()) == 0:
		rows = append(rows, r.NewStyle().Faint(true).Render("No bot games yet."))
	default:
		rows = append(rows, m.botGamesTable.View())
	}
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m botModel) botHeaderRows() []string {
	r := m.ctx.renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1)

	infoStyle := r.NewStyle().Foreground(lipgloss.Color("252"))
	highlightStyle := r.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	noticeStyle := r.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)

	rows := []string{
		titleStyle.Render(botPageTitle),
		"",
		infoStyle.Render("Game ID: ") + highlightStyle.Render(m.currentBotGame.ID()),
		infoStyle.Render("Level: ") + highlightStyle.Render(strconv.Itoa(m.currentBotGame.BotLevel())),
		infoStyle.Render(botStatusLine(m.currentBotGame)),
	}
	if turn := botTurnLine(m.currentBotGame, m.botMoving); turn != "" {
		rows = append(rows, highlightStyle.Render(turn))
	}
	if m.botNotice != "" {
		rows = append(rows, noticeStyle.Render(m.botNotice))
	}
	return rows
}

func (m botModel) viewBotInProgress() string {
	helpStyle := m.ctx.renderer.NewStyle().Foreground(lipgloss.Color("241"))
	rows := append(m.botHeaderRows(), "", m.renderBotBoardFromFEN(), "", helpStyle.Render(botHelpMove), helpStyle.Render(botHelpResign))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m botModel) viewBotFinished() string {
	helpStyle := m.ctx.renderer.NewStyle().Foreground(lipgloss.Color("241"))
	rows := append(m.botHeaderRows(), "", m.renderBotBoardFromFEN(), "", helpStyle.Render("Press esc to return to bot lobby."))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
