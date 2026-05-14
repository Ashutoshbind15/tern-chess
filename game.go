package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/Ashutoshbind15/ssh-chess/managers"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
	"github.com/notnil/chess"
)

const (
	gamePageTitle      = "Game Page"
	gameHelpCreate     = "Create a game: ctrl+n"
	gameHelpJoinRandom = "Join a random game: ctrl+r"
	gameHelpJoinByID   = "Join by ID: type the game ID below and press enter"
	gameHelpMove       = "Make a move: click a piece then a square, or type UCI like e2e4"
	gameNoGame         = "No game"
	gameHelpTimeSelect = "Select time: [1] 1 min  [3] 3 min  [5] 5 min"
)

type gameModel struct {
	ctx *Context

	gameJoinInput textinput.Model
	moveInput     textinput.Model

	currentGame         *managers.Game
	gameNotice          string
	selectedTimeControl TimeControlChoice
	whiteTimeLeft       time.Duration
	blackTimeLeft       time.Duration
	selected            string
	possibleMoves       []string
}

func newGameModel(ctx *Context) gameModel {
	gameJoinInput := common.InitTextInput()
	applyRendererTextInputStyles(&gameJoinInput, ctx.renderer)
	gameJoinInput.Prompt = "game id> "
	gameJoinInput.Placeholder = "abc123"
	gameJoinInput.Width = textInputViewWidth

	moveInput := common.InitTextInput()
	applyRendererTextInputStyles(&moveInput, ctx.renderer)
	moveInput.Prompt = "move> "
	moveInput.Placeholder = "e2e4"
	moveInput.Width = textInputViewWidth

	return gameModel{
		ctx:                 ctx,
		gameJoinInput:       gameJoinInput,
		moveInput:           moveInput,
		currentGame:         gameManager.GameForPlayer(ctx.fingerPrint),
		selectedTimeControl: NoTimeControl,
	}
}

func (m gameModel) Init() tea.Cmd { return nil }

func (m gameModel) Activate() (gameModel, tea.Cmd) {
	if m.currentGame == nil {
		return m, m.gameJoinInput.Focus()
	}
	if m.currentGame.Status() == managers.GameStatusInProgress {
		return m, m.moveInput.Focus()
	}
	return m, nil
}

func notifyOpponentJoined(gameID string, joinerFingerprint string) {
	opp := gameManager.OpponentFingerprint(gameID, joinerFingerprint)
	if opp == "" {
		return
	}
	if prog := sessionManager.GetProgram(opp); prog != nil {
		prog.Send(opponentJoinedGameMsg{
			opponentName: gameManager.PlayerUsername(joinerFingerprint),
		})
	}
}

func notifyOpponentMoved(gameID string, moverFingerprint string, move string) {
	opp := gameManager.OpponentFingerprint(gameID, moverFingerprint)
	if opp == "" {
		return
	}
	if prog := sessionManager.GetProgram(opp); prog != nil {
		prog.Send(gameUpdatedMsg{move: move})
	}
}

// persistAndRemoveGameCmd persists a finished game and clears it from
// memory off the Bubble Tea event loop. The DB write happens in a goroutine
// so Update() doesn't block waiting on Postgres. Returns a nil msg because
// each affected player is notified via prog.Send(gamesRefreshMsg{}) inside
// the goroutine (the gamesRefreshMsg fanout reaches both players, not just
// the caller, so a per-program msg return wouldn't be enough).
func persistAndRemoveGameCmd(gameID string) tea.Cmd {
	return func() tea.Msg {
		record := gameManager.BuildGameRecord(gameID)
		if err := dataManager.AddGame(record); err == nil {
			gameManager.RemoveGame(gameID)
			for _, fp := range []string{record.WhiteFingerprint, record.BlackFingerprint} {
				if prog := sessionManager.GetProgram(fp); prog != nil {
					prog.Send(gamesRefreshMsg{})
				}
			}
		}
		return nil
	}
}

func gamesRefreshCmd() tea.Cmd {
	return func() tea.Msg { return gamesRefreshMsg{} }
}

func navigateToChatCmdGame() tea.Cmd {
	return func() tea.Msg { return navigateMsg{page: PageChat} }
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func formatClock(whiteTime, blackTime time.Duration, playerColor chess.Color) string {
	whiteStr := formatDuration(whiteTime)
	blackStr := formatDuration(blackTime)

	if playerColor == chess.White {
		return "Time | You: " + whiteStr + "  Opp: " + blackStr
	}
	return "Time | You: " + blackStr + "  Opp: " + whiteStr
}

func gameStatusLine(status string) string {
	switch status {
	case managers.GameStatusWaiting:
		return "Status: waiting for an opponent."
	case managers.GameStatusInProgress:
		return "Status: in progress - play when it is your turn."
	case managers.GameStatusFinished:
		return "Status: finished."
	default:
		if status == "" {
			return ""
		}
		return "Status: " + status
	}
}

func convertToChessboardPosition(x, y int, colorIsWhite bool) string {
	if colorIsWhite {
		return fmt.Sprintf("%c%d", 'a'+x, 8-y)
	}
	return fmt.Sprintf("%c%d", 'h'-x, y+1)
}

func parseBoardFENToString(fen string) [8][8]string {
	var board [8][8]string
	fields := strings.Fields(fen)
	if len(fields) == 0 {
		return board
	}
	rows := strings.Split(fields[0], "/")
	for rowIndex, row := range rows {
		fileIndex := 0
		for _, square := range row {
			if square >= '1' && square <= '8' {
				emptySquares := int(square - '0')
				for i := 0; i < emptySquares && fileIndex < 8; i++ {
					board[rowIndex][fileIndex] = " "
					fileIndex++
				}
				continue
			}
			if fileIndex < 8 {
				board[rowIndex][fileIndex] = string(square)
				fileIndex++
			}
		}
	}
	return board
}

// Unicode chess symbols for the board
func boardGlyph(piece rune) string {
	if piece == 0 || piece == ' ' {
		return " "
	}
	switch piece {
	case 'K':
		return "\u2654"
	case 'Q':
		return "\u2655"
	case 'R':
		return "\u2656"
	case 'B':
		return "\u2657"
	case 'N':
		return "\u2658"
	case 'P':
		return "\u2659"
	case 'k':
		return "\u265a"
	case 'q':
		return "\u265b"
	case 'r':
		return "\u265c"
	case 'b':
		return "\u265d"
	case 'n':
		return "\u265e"
	case 'p':
		return "\u265f"
	default:
		return string(piece)
	}
}

// renderChessBoard renders an 8x8 board from a FEN string with mouse-zone
// markers so clicks can be mapped back to algebraic squares. It is shared
// between the multiplayer and bot pages so the zone layout is guaranteed
// to be identical for both.
func renderChessBoard(r *lipgloss.Renderer, z *zone.Manager, fen string, colorIsWhite bool, selected string, possibleMoves []string) string {
	board := parseBoardFENToString(fen)
	flipped := !colorIsWhite

	cellStyle := r.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))

	rows := make([]string, 8)
	for i := 0; i < 8; i++ {
		sourceRow := i
		if flipped {
			sourceRow = 7 - i
		}

		cells := make([]string, 8)
		for j := 0; j < 8; j++ {
			sourceCol := j
			if flipped {
				sourceCol = 7 - j
			}

			pos := convertToChessboardPosition(j, i, colorIsWhite)
			glyph := boardGlyph(rune(board[sourceRow][sourceCol][0]))

			var cell string
			if selected == pos {
				cell = cellStyle.Copy().BorderForeground(lipgloss.Color("190")).Render(glyph)
			} else if containsSquare(possibleMoves, pos) {
				cell = cellStyle.Copy().BorderForeground(lipgloss.Color("229")).Render(glyph)
			} else {
				cell = cellStyle.Render(glyph)
			}

			cells[j] = z.Mark(pos, cell)
		}
		rows[i] = lipgloss.JoinHorizontal(lipgloss.Left, cells...)
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m gameModel) renderBoardFromFEN() string {
	fen := m.currentGame.Game().FEN()
	colorIsWhite := m.currentGame.PlayerColor(m.ctx.fingerPrint) != chess.Black
	return renderChessBoard(m.ctx.renderer, m.ctx.zone, fen, colorIsWhite, m.selected, m.possibleMoves)
}

func containsSquare(squares []string, target string) bool {
	for _, s := range squares {
		if s == target {
			return true
		}
	}
	return false
}

func gameTurnLine(game *managers.Game, fingerprint string) string {
	if game == nil {
		return ""
	}

	playerColor := game.PlayerColor(fingerprint)
	if playerColor == chess.NoColor {
		return ""
	}

	colorName := strings.ToLower(playerColor.Name())
	if game.Status() != managers.GameStatusInProgress {
		return "You are " + colorName + "."
	}
	if game.IsPlayersTurn(fingerprint) {
		return "You are " + colorName + ". Your turn."
	}

	return "You are " + colorName + ". " + game.Turn().Name() + " to move."
}

// Update is the entry point for the game page. It first handles
// page-scoped messages (opponent joined / opponent moved, clock ticks,
// time forfeit) and then routes to a state-specific handler based on whether
// the player has a game and what state that game is in. Each sub-handler can
// assume the invariants for its state, so we don't have to defensively
// re-check them.
func (m gameModel) Update(msg tea.Msg) (gameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case opponentJoinedGameMsg:
		m.currentGame = gameManager.GameForPlayer(m.ctx.fingerPrint)
		opponent := msg.opponentName
		if opponent == "" {
			opponent = "Opponent"
		}
		m.gameNotice = opponent + " joined. Game on."
		return m, m.moveInput.Focus()
	case gameUpdatedMsg:
		if m.currentGame == nil {
			return m, nil
		}
		if msg.move != "" {
			m.gameNotice = "Opponent played " + msg.move + "."
		}
		return m, nil
	case ClockUpdateMsg:
		if m.currentGame != nil && m.currentGame.ID() == msg.GameID {
			m.whiteTimeLeft = msg.WhiteTime
			m.blackTimeLeft = msg.BlackTime
		}
		return m, nil
	case TimeForfeitMsg:
		if m.currentGame == nil || m.currentGame.ID() != msg.GameID {
			return m, nil
		}
		if msg.LoserColor == chess.White {
			m.gameNotice = "White ran out of time. Black wins!"
		} else {
			m.gameNotice = "Black ran out of time. White wins!"
		}
		return m, gamesRefreshCmd()
	}

	if m.currentGame == nil {
		return m.updateGameLobby(msg)
	}
	switch m.currentGame.Status() {
	case managers.GameStatusWaiting:
		return m.updateGameWaiting(msg)
	case managers.GameStatusInProgress:
		return m.updateGameInProgress(msg)
	case managers.GameStatusFinished:
		return m.updateGameFinished(msg)
	}
	return m, nil
}

// updateGameLobby handles input when the player is on the game page but
// not yet in a game: pick a time control, then create, join random, or join by ID.
func (m gameModel) updateGameLobby(msg tea.Msg) (gameModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, navigateToChatCmdGame()
		case "1":
			m.selectedTimeControl = TimeControl1
			return m, nil
		case "3":
			m.selectedTimeControl = TimeControl3
			return m, nil
		case "5":
			m.selectedTimeControl = TimeControl5
			return m, nil
		case "ctrl+n":
			if m.selectedTimeControl == NoTimeControl {
				m.gameNotice = "Select a time control first (1, 3, or 5)."
				return m, nil
			}
			gameID, err := gameManager.CreateGame(m.ctx.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.ctx.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Created " + m.selectedTimeControl.ToGameTimeControl().String() + " game " + gameID + ". Share the ID with your opponent."
			return m, nil
		case "ctrl+r":
			if m.selectedTimeControl == NoTimeControl {
				m.gameNotice = "Select a time control first (1, 3, or 5)."
				return m, nil
			}
			gameID, err := gameManager.JoinRandomGame(m.ctx.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.ctx.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined " + m.currentGame.TimeControl().String() + " game " + gameID + "."
			notifyOpponentJoined(gameID, m.ctx.fingerPrint)
			return m, m.moveInput.Focus()
		case "enter":
			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				return m, nil
			}
			if _, err := gameManager.JoinGame(m.ctx.fingerPrint, gameID); err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.ctx.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined " + m.currentGame.TimeControl().String() + " game " + gameID + "."
			notifyOpponentJoined(gameID, m.ctx.fingerPrint)
			if m.currentGame.Status() == managers.GameStatusInProgress {
				return m, m.moveInput.Focus()
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.gameJoinInput, cmd = m.gameJoinInput.Update(msg)
	return m, cmd
}

// updateGameWaiting handles input while waiting for an opponent to join.
// Nothing to do here besides letting the player back out.
func (m gameModel) updateGameWaiting(msg tea.Msg) (gameModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, navigateToChatCmdGame()
	}
	return m, nil
}

// updateGameInProgress handles input during an active game: typing /
// submitting a move via the move input, and mouse clicks on the board.
func (m gameModel) updateGameInProgress(msg tea.Msg) (gameModel, tea.Cmd) {
	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		return m.handleBoardMouse(mouseMsg)
	}

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m.selected = ""
			m.possibleMoves = nil
			return m, navigateToChatCmdGame()
		case "enter":
			move := strings.ToLower(strings.TrimSpace(m.moveInput.Value()))
			if move == "" {
				return m, nil
			}
			game, err := gameManager.MakeMove(m.ctx.fingerPrint, move)
			if err != nil {
				m.gameNotice = "Move rejected: " + err.Error()
				return m, nil
			}
			m.currentGame = game
			m.moveInput.SetValue("")
			m.gameNotice = "Played " + move + "."

			if game.Status() == managers.GameStatusFinished {
				gameID := game.ID()
				oppFP := gameManager.OpponentFingerprint(gameID, m.ctx.fingerPrint)
				if oppFP != "" {
					if prog := sessionManager.GetProgram(oppFP); prog != nil {
						prog.Send(gameUpdatedMsg{move: move})
					}
				}
				return m, persistAndRemoveGameCmd(gameID)
			}

			notifyOpponentMoved(game.ID(), m.ctx.fingerPrint, move)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.moveInput, cmd = m.moveInput.Update(msg)
	return m, cmd
}

func (m gameModel) handleBoardMouse(msg tea.MouseMsg) (gameModel, tea.Cmd) {
	if msg.Action != tea.MouseActionRelease || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}

	if !m.currentGame.IsPlayersTurn(m.ctx.fingerPrint) {
		m.selected = ""
		m.possibleMoves = nil
		return m, nil
	}

	colorIsWhite := m.currentGame.PlayerColor(m.ctx.fingerPrint) == chess.White
	doesClick := false

	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			pos := convertToChessboardPosition(j, i, colorIsWhite)
			if m.ctx.zone.Get(pos).InBounds(msg) {
				doesClick = true

				if m.selected == "" {
					m.selected = pos
					fenOpt, err := chess.FEN(m.currentGame.Game().FEN())
					if err != nil {
						return m, nil
					}
					clientChessClient := chess.NewGame(chess.UseNotation(chess.UCINotation{}), fenOpt)
					possibleMoves := clientChessClient.ValidMoves()

					validMovesFromSelected := []string{}
					for _, move := range possibleMoves {
						moveStr := move.String()
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
					game, err := gameManager.MakeMove(m.ctx.fingerPrint, moveUCI)
					if err != nil {
						game2, perr := gameManager.MakeMove(m.ctx.fingerPrint, moveUCI+"q")
						if perr != nil {
							m.gameNotice = "Move rejected: " + err.Error()
						} else {
							game = game2
						}
					}

					m.selected = ""
					m.possibleMoves = nil

					if game == nil {
						return m, nil
					}

					m.currentGame = game
					m.gameNotice = "Played " + moveUCI + "."

					if game.Status() == managers.GameStatusFinished {
						gameID := game.ID()
						oppFP := gameManager.OpponentFingerprint(gameID, m.ctx.fingerPrint)
						if oppFP != "" {
							if prog := sessionManager.GetProgram(oppFP); prog != nil {
								prog.Send(gameUpdatedMsg{move: moveUCI})
							}
						}
						return m, persistAndRemoveGameCmd(gameID)
					}

					notifyOpponentMoved(game.ID(), m.ctx.fingerPrint, moveUCI)
					return m, nil
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

// updateGameFinished handles input after the game ended.
func (m gameModel) updateGameFinished(msg tea.Msg) (gameModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, navigateToChatCmdGame()
	}
	return m, nil
}

func (m gameModel) getGameBoard() string {
	if m.currentGame != nil && m.currentGame.Game() != nil {
		return m.renderBoardFromFEN()
	}
	return gameNoGame
}

func (m gameModel) View() string {
	if m.currentGame == nil {
		return m.viewGameLobby()
	}
	switch m.currentGame.Status() {
	case managers.GameStatusWaiting:
		return m.viewGameWaiting()
	case managers.GameStatusInProgress:
		return m.viewGameInProgress()
	case managers.GameStatusFinished:
		return m.viewGameFinished()
	}
	return ""
}

func (m gameModel) gamePageCommonRows() []string {
	r := m.ctx.renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1)

	helpStyle := r.NewStyle().Foreground(lipgloss.Color("241"))
	highlightStyle := r.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	infoStyle := r.NewStyle().Foreground(lipgloss.Color("252"))

	rows := []string{
		titleStyle.Render(gamePageTitle),
		"",
	}
	if m.selectedTimeControl == NoTimeControl {
		rows = append(rows, helpStyle.Render(gameHelpTimeSelect))
	} else {
		rows = append(rows, infoStyle.Render("Time control: ")+highlightStyle.Render(strconv.Itoa(int(m.selectedTimeControl))+" min")+helpStyle.Render("  (press 1/3/5 to change)"))
	}
	rows = append(rows, "", helpStyle.Render(gameHelpCreate), helpStyle.Render(gameHelpJoinRandom), helpStyle.Render(gameHelpJoinByID), "", m.gameJoinInput.View())
	return rows
}

func (m gameModel) viewGameLobby() string {
	rows := m.gamePageCommonRows()
	if m.gameNotice != "" {
		noticeStyle := m.ctx.renderer.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)
		rows = append(rows, "", noticeStyle.Render(m.gameNotice))
	}
	rows = append(rows, "", m.ctx.renderer.NewStyle().Faint(true).Render(gameNoGame))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// gameHeaderRows is the shared header used by all in-game views (status,
// time control, live clocks while in progress, notices).
func (m gameModel) gameHeaderRows() []string {
	r := m.ctx.renderer

	titleStyle := r.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Padding(0, 1)

	infoStyle := r.NewStyle().Foreground(lipgloss.Color("252"))
	highlightStyle := r.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	noticeStyle := r.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57")).Padding(0, 1)

	rows := []string{
		titleStyle.Render(gamePageTitle),
		"",
		infoStyle.Render("Game ID: ") + highlightStyle.Render(m.currentGame.ID()),
		infoStyle.Render(gameStatusLine(m.currentGame.Status())),
	}
	if m.currentGame.TimeControl() != 0 {
		rows = append(rows, infoStyle.Render("Time control: ")+highlightStyle.Render(m.currentGame.TimeControl().String()))
	}
	if turnLine := gameTurnLine(m.currentGame, m.ctx.fingerPrint); turnLine != "" {
		rows = append(rows, highlightStyle.Render(turnLine))
	}
	if m.currentGame.Status() == managers.GameStatusInProgress {
		playerColor := m.currentGame.PlayerColor(m.ctx.fingerPrint)
		if playerColor != chess.NoColor {
			rows = append(rows, infoStyle.Render(formatClock(m.whiteTimeLeft, m.blackTimeLeft, playerColor)))
		}
	}
	if m.gameNotice != "" {
		rows = append(rows, noticeStyle.Render(m.gameNotice))
	}
	return rows
}

func (m gameModel) viewGameWaiting() string {
	helpStyle := m.ctx.renderer.NewStyle().Foreground(lipgloss.Color("241"))
	rows := append(m.gameHeaderRows(), "", m.getGameBoard(), "", helpStyle.Render("Waiting for an opponent. Share the game ID above."))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m gameModel) viewGameInProgress() string {
	helpStyle := m.ctx.renderer.NewStyle().Foreground(lipgloss.Color("241"))
	rows := append(m.gameHeaderRows(), "", m.getGameBoard(), "", helpStyle.Render(gameHelpMove), m.moveInput.View())
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m gameModel) viewGameFinished() string {
	rows := append(m.gameHeaderRows(), "", m.getGameBoard())
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
