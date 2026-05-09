package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ashutoshbind15/ssh-chess/managers"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/notnil/chess"
)

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

func persistAndRemoveGame(gameID string) {
	record := gameManager.BuildGameRecord(gameID)
	if err := dataManager.AddGame(record); err == nil {
		gameManager.RemoveGame(gameID)
		// Both players need their intro games table refreshed.
		for _, fp := range []string{record.WhiteFingerprint, record.BlackFingerprint} {
			if prog := sessionManager.GetProgram(fp); prog != nil {
				prog.Send(gamesRefreshMsg{})
			}
		}
	}
}

const (
	gamePageTitle      = "Game Page"
	gameHelpCreate     = "Create a game: ctrl+n"
	gameHelpJoinRandom = "Join a random game: ctrl+r"
	gameHelpJoinByID   = "Join by ID: type the game ID below and press enter"
	gameHelpMove       = "Make a move: type UCI like e2e4 and press enter"
	gameNoGame         = "No game"
	gameHelpTimeSelect = "Select time: [1] 1 min  [3] 3 min  [5] 5 min"
)

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	totalSeconds := int(d.Seconds())
	minutes := totalSeconds / 60
	seconds := totalSeconds % 60
	if d < time.Minute {
		return fmt.Sprintf("%d:%02d", minutes, seconds)
	}
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

func gamePageCommonRows(m model) []string {
	rows := []string{
		gamePageTitle,
		"",
	}
	if m.selectedTimeControl == NoTimeControl {
		rows = append(rows, gameHelpTimeSelect)
	} else {
		rows = append(rows, "Time control: "+strconv.Itoa(int(m.selectedTimeControl))+" min  (press 1/3/5 to change)")
	}
	rows = append(rows, "", gameHelpCreate, gameHelpJoinRandom, gameHelpJoinByID, "", m.gameJoinInput.View())
	return rows
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

func parseBoardFEN(fen string) [8][8]rune {
	var board [8][8]rune
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
					board[rowIndex][fileIndex] = ' '
					fileIndex++
				}
				continue
			}
			if fileIndex < 8 {
				board[rowIndex][fileIndex] = square
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

func renderBoardFromFEN(fen string, flipped bool, r *lipgloss.Renderer) string {
	board := parseBoardFEN(fen)

	cellStyle := r.NewStyle().
		Padding(0, 1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240"))
	fileLabel := r.NewStyle().Width(5).Align(lipgloss.Center)
	rankLabel := r.NewStyle().Width(3).Align(lipgloss.Center)

	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	if flipped {
		files = []string{"h", "g", "f", "e", "d", "c", "b", "a"}
	}

	header := []string{rankLabel.Render("")}
	for _, file := range files {
		header = append(header, fileLabel.Render(file))
	}
	header = append(header, rankLabel.Render(""))
	headerLine := lipgloss.JoinHorizontal(lipgloss.Top, header...)

	rows := []string{headerLine}
	for displayRow := 0; displayRow < 8; displayRow++ {
		sourceRow := displayRow
		rank := 8 - displayRow
		if flipped {
			sourceRow = 7 - displayRow
			rank = displayRow + 1
		}

		cells := []string{rankLabel.Render(strconv.Itoa(rank))}
		for displayCol := 0; displayCol < 8; displayCol++ {
			sourceCol := displayCol
			if flipped {
				sourceCol = 7 - displayCol
			}
			cells = append(cells, cellStyle.Render(boardGlyph(board[sourceRow][sourceCol])))
		}
		cells = append(cells, rankLabel.Render(strconv.Itoa(rank)))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Center, cells...))
	}

	rows = append(rows, headerLine)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
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

// UpdateGame is the entry point for the game page. It first handles
// page-scoped messages (opponent joined / opponent moved, clock ticks,
// time forfeit) and then routes to a state-specific handler based on whether
// the player has a game and what state that game is in. Each sub-handler can
// assume the invariants for its state, so we don't have to defensively
// re-check them.
func (m model) UpdateGame(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case opponentJoinedGameMsg:
		m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
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
	case managers.ClockUpdateMsg:
		if m.currentGame != nil && m.currentGame.ID() == msg.GameID {
			m.whiteTimeLeft = msg.WhiteTime
			m.blackTimeLeft = msg.BlackTime
		}
		return m, nil
	case managers.TimeForfeitMsg:
		if m.currentGame == nil || m.currentGame.ID() != msg.GameID {
			return m, nil
		}
		if msg.LoserColor == chess.White {
			m.gameNotice = "White ran out of time. Black wins!"
		} else {
			m.gameNotice = "Black ran out of time. White wins!"
		}
		m.gamesLoading = true
		return m, loadGamesCmd(m.fingerPrint)
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
func (m model) updateGameLobby(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, m.chatTextarea.Focus()
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
			gameID, err := gameManager.CreateGame(m.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Created " + m.selectedTimeControl.ToGameTimeControl().String() + " game " + gameID + ". Share the ID with your opponent."
			return m, nil
		case "ctrl+r":
			if m.selectedTimeControl == NoTimeControl {
				m.gameNotice = "Select a time control first (1, 3, or 5)."
				return m, nil
			}
			gameID, err := gameManager.JoinRandomGame(m.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined " + m.currentGame.TimeControl().String() + " game " + gameID + "."
			notifyOpponentJoined(gameID, m.fingerPrint)
			return m, m.moveInput.Focus()
		case "enter":
			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				return m, nil
			}
			if _, err := gameManager.JoinGame(m.fingerPrint, gameID); err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}
			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined " + m.currentGame.TimeControl().String() + " game " + gameID + "."
			notifyOpponentJoined(gameID, m.fingerPrint)
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
func (m model) updateGameWaiting(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m = m.navigateTo(PageChat)
		return m, m.chatTextarea.Focus()
	}
	return m, nil
}

// updateGameInProgress handles input during an active game: typing /
// submitting a move via the move input.
func (m model) updateGameInProgress(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, m.chatTextarea.Focus()
		case "enter":
			move := strings.ToLower(strings.TrimSpace(m.moveInput.Value()))
			if move == "" {
				return m, nil
			}
			game, err := gameManager.MakeMove(m.fingerPrint, move)
			if err != nil {
				m.gameNotice = "Move rejected: " + err.Error()
				return m, nil
			}
			m.currentGame = game
			m.moveInput.SetValue("")
			m.gameNotice = "Played " + move + "."

			if game.Status() == managers.GameStatusFinished {
				gameID := game.ID()
				oppFP := gameManager.OpponentFingerprint(gameID, m.fingerPrint)
				persistAndRemoveGame(gameID)
				if oppFP != "" {
					if prog := sessionManager.GetProgram(oppFP); prog != nil {
						prog.Send(gameUpdatedMsg{move: move})
					}
				}
				m.gamesLoading = true
				return m, loadGamesCmd(m.fingerPrint)
			}

			notifyOpponentMoved(game.ID(), m.fingerPrint, move)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.moveInput, cmd = m.moveInput.Update(msg)
	return m, cmd
}

// updateGameFinished handles input after the game ended.
func (m model) updateGameFinished(msg tea.Msg) (model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		m = m.navigateTo(PageChat)
		return m, m.chatTextarea.Focus()
	}
	return m, nil
}

func (m model) getGameBoard() string {
	if m.currentGame != nil && m.currentGame.Game() != nil {
		return renderBoardFromFEN(
			m.currentGame.Game().FEN(),
			m.currentGame.PlayerColor(m.fingerPrint) == chess.Black,
			m.renderer,
		)
	}
	return gameNoGame
}

func (m model) ViewGame() string {
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

func (m model) viewGameLobby() string {
	rows := gamePageCommonRows(m)
	if m.gameNotice != "" {
		rows = append(rows, "", m.gameNotice)
	}
	rows = append(rows, "", gameNoGame)
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// gameHeaderRows is the shared header used by all in-game views (status,
// time control, live clocks while in progress, notices).
func (m model) gameHeaderRows() []string {
	rows := []string{
		gamePageTitle,
		"",
		"Game ID: " + m.currentGame.ID(),
		gameStatusLine(m.currentGame.Status()),
	}
	if m.currentGame.TimeControl() != 0 {
		rows = append(rows, "Time control: "+m.currentGame.TimeControl().String())
	}
	if turnLine := gameTurnLine(m.currentGame, m.fingerPrint); turnLine != "" {
		rows = append(rows, turnLine)
	}
	if m.currentGame.Status() == managers.GameStatusInProgress {
		playerColor := m.currentGame.PlayerColor(m.fingerPrint)
		if playerColor != chess.NoColor {
			rows = append(rows, formatClock(m.whiteTimeLeft, m.blackTimeLeft, playerColor))
		}
	}
	if m.gameNotice != "" {
		rows = append(rows, m.gameNotice)
	}
	return rows
}

func (m model) viewGameWaiting() string {
	rows := append(m.gameHeaderRows(), "", m.getGameBoard(), "", "Waiting for an opponent. Share the game ID above.")
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) viewGameInProgress() string {
	rows := append(m.gameHeaderRows(), "", m.getGameBoard(), "", gameHelpMove, m.moveInput.View())
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

func (m model) viewGameFinished() string {
	rows := append(m.gameHeaderRows(), "", m.getGameBoard())
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}