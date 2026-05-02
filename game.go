package main

import (
	"strconv"
	"strings"

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
		prog.Send(opponentJoinedGameMsg{})
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

const (
	gamePageTitle      = "Game Page"
	gameHelpCreate     = "Create a game: ctrl+n"
	gameHelpJoinRandom = "Join a random game: ctrl+r"
	gameHelpJoinByID   = "Join by ID: type the game ID below and press enter"
	gameHelpMove       = "Make a move: type UCI like e2e4 and press enter"
	gameNoGame         = "No game"
)

var fenPieceToGlyph = map[rune]string{
	'K': "♔",
	'Q': "♕",
	'R': "♖",
	'B': "♗",
	'N': "♘",
	'P': "♙",
	'k': "♚",
	'q': "♛",
	'r': "♜",
	'b': "♝",
	'n': "♞",
	'p': "♟",
}

func gamePageCommonRows(m model) []string {
	return []string{
		gamePageTitle,
		"",
		gameHelpCreate,
		gameHelpJoinRandom,
		gameHelpJoinByID,
		"",
		m.gameJoinInput.View(),
	}
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

func boardGlyph(piece rune) string {
	if glyph, ok := fenPieceToGlyph[piece]; ok {
		return glyph
	}
	return " "
}

func renderBoardFromFEN(fen string, flipped bool) string {
	board := parseBoardFEN(fen)
	lightSquare := lipgloss.NewStyle().
		Width(3).
		Align(lipgloss.Center).
		Background(lipgloss.Color("252")).
		Foreground(lipgloss.Color("0"))
	darkSquare := lipgloss.NewStyle().
		Width(3).
		Align(lipgloss.Center).
		Background(lipgloss.Color("240")).
		Foreground(lipgloss.Color("15"))
	fileLabel := lipgloss.NewStyle().
		Width(3).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("245"))
	rankLabel := lipgloss.NewStyle().
		Width(2).
		Align(lipgloss.Center).
		Foreground(lipgloss.Color("245"))

	files := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	if flipped {
		files = []string{"h", "g", "f", "e", "d", "c", "b", "a"}
	}

	header := []string{rankLabel.Render("")}
	for _, file := range files {
		header = append(header, fileLabel.Render(file))
	}
	header = append(header, rankLabel.Render(""))

	rows := []string{lipgloss.JoinHorizontal(lipgloss.Left, header...)}
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

			squareStyle := lightSquare
			if (sourceRow+sourceCol)%2 == 1 {
				squareStyle = darkSquare
			}
			cells = append(cells, squareStyle.Render(boardGlyph(board[sourceRow][sourceCol])))
		}
		cells = append(cells, rankLabel.Render(strconv.Itoa(rank)))
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, cells...))
	}

	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Left, header...))
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

func (m model) UpdateGame(msg tea.Msg) (model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m = m.navigateTo(PageChat)
			return m, nil
		case "ctrl+n":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}
			if m.currentGame != nil {
				m.gameNotice = "You are already in a game."
				return m, nil
			}

			gameID, err := gameManager.CreateGame(m.fingerPrint)
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Created game " + gameID + ". Share the ID with your opponent."
			return m, nil
		case "ctrl+r":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}
			if m.currentGame != nil {
				m.gameNotice = "You are already in a game."
				return m, nil
			}

			gameID, err := gameManager.JoinRandomGame(m.fingerPrint)
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined game " + gameID + "."
			notifyOpponentJoined(gameID, m.fingerPrint)
			return m, nil
		case "enter":
			if m.player == nil {
				m = m.navigateTo(PageIntro)
				return m, nil
			}
			if m.currentGame != nil && m.currentGame.Status() == managers.GameStatusInProgress {
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
				notifyOpponentMoved(game.ID(), m.fingerPrint, move)
				return m, nil
			}
			if m.currentGame != nil {
				return m, nil
			}

			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				return m, nil
			}

			_, err := gameManager.JoinGame(m.fingerPrint, gameID)
			if err != nil {
				m.gameNotice = err.Error()
				return m, nil
			}

			m.currentGame = gameManager.GameForPlayer(m.fingerPrint)
			m.gameJoinInput.SetValue("")
			m.moveInput.SetValue("")
			m.gameNotice = "Joined game " + gameID + "."
			notifyOpponentJoined(gameID, m.fingerPrint)
			return m, nil
		}
	}

	var cmd tea.Cmd
	if m.currentGame == nil {
		m.gameJoinInput, cmd = m.gameJoinInput.Update(msg)
		return m, cmd
	}
	if m.currentGame.Status() == managers.GameStatusInProgress {
		m.moveInput, cmd = m.moveInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) getGameBoard() string {
	if m.currentGame != nil && m.currentGame.Game() != nil {
		return renderBoardFromFEN(
			m.currentGame.Game().FEN(),
			m.currentGame.PlayerColor(m.fingerPrint) == chess.Black,
		)
	}
	return gameNoGame
}

func (m model) ViewGame() string {
	if m.currentGame == nil {
		rows := gamePageCommonRows(m)
		if m.gameNotice != "" {
			rows = append(rows, "", m.gameNotice)
		}
		rows = append(rows, "", gameNoGame)
		return lipgloss.JoinVertical(lipgloss.Left, rows...)
	}

	status := m.currentGame.Status()
	rows := []string{
		gamePageTitle,
		"",
		"Game ID: " + m.currentGame.ID(),
		gameStatusLine(status),
	}

	if turnLine := gameTurnLine(m.currentGame, m.fingerPrint); turnLine != "" {
		rows = append(rows, turnLine)
	}
	if m.gameNotice != "" {
		rows = append(rows, m.gameNotice)
	}

	rows = append(rows, "", m.getGameBoard())
	if status == managers.GameStatusWaiting {
		rows = append(rows, "", "Waiting for an opponent. Share the game ID above.")
	}
	if status == managers.GameStatusInProgress {
		rows = append(rows, "", gameHelpMove, m.moveInput.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}
