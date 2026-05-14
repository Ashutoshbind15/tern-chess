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

// gameModel holds an immutable Snapshot of the current game rather than a
// pointer into the manager. Every mutation goes through a tea.Cmd that
// produces a fresh snapshot via the manager (under its mutex), which means
// View() never reads concurrently with a writer goroutine. The
// movePending flag provides UI-level backpressure so the user can't queue
// a second move while the first is in flight.
type gameModel struct {
	ctx *Context

	gameJoinInput textinput.Model
	moveInput     textinput.Model

	snapshot            *managers.Snapshot
	gameNotice          string
	selectedTimeControl TimeControlChoice
	whiteTimeLeft       time.Duration
	blackTimeLeft       time.Duration
	selected            string
	possibleMoves       []string
	movePending         bool
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
		snapshot:            gameManager.SnapshotForPlayer(ctx.fingerPrint),
		selectedTimeControl: NoTimeControl,
	}
}

func (m gameModel) Init() tea.Cmd { return nil }

// withSnapshot installs s as the model's snapshot and re-seeds the local
// clock fields. The clock ticker writes m.whiteTimeLeft/m.blackTimeLeft
// every 250ms, but in between ticks we want the post-move clocks visible
// immediately, so any code path that produces a new snapshot funnels
// through here.
func (m gameModel) withSnapshot(s *managers.Snapshot) gameModel {
	m.snapshot = s
	if s != nil {
		m.whiteTimeLeft = s.WhiteTimeLeft
		m.blackTimeLeft = s.BlackTimeLeft
	}
	return m
}

func (m gameModel) Activate() (gameModel, tea.Cmd) {
	m = m.withSnapshot(gameManager.SnapshotForPlayer(m.ctx.fingerPrint))
	if m.snapshot == nil {
		return m, m.gameJoinInput.Focus()
	}
	if m.snapshot.Status == managers.GameStatusInProgress {
		return m, m.moveInput.Focus()
	}
	return m, nil
}

// --- Messages produced by game-manager cmds ----------------------------------

type lobbyActionKind int

const (
	lobbyActionCreate lobbyActionKind = iota
	lobbyActionJoin
	lobbyActionJoinRandom
)

// gameLobbyResultMsg is the response to any of the three lobby actions
// (create / join-by-id / join-random). On success snapshot is non-nil; on
// failure err is non-nil and snapshot is nil.
type gameLobbyResultMsg struct {
	snapshot *managers.Snapshot
	kind     lobbyActionKind
	err      error
}

// moveAppliedMsg is the response to a MakeMove call. The model interprets
// err == nil && snapshot != nil as a successful move and updates state /
// dispatches follow-ups.
type moveAppliedMsg struct {
	snapshot *managers.Snapshot
	move     string
	err      error
}

// --- Cmds --------------------------------------------------------------------

func createGameCmd(fingerprint string, tc managers.TimeControl) tea.Cmd {
	return func() tea.Msg {
		snap, err := gameManager.CreateGame(fingerprint, tc)
		return gameLobbyResultMsg{snapshot: snap, kind: lobbyActionCreate, err: err}
	}
}

func joinGameCmd(fingerprint, gameID string) tea.Cmd {
	return func() tea.Msg {
		snap, err := gameManager.JoinGame(fingerprint, gameID)
		if err == nil && snap != nil {
			notifyOpponentJoined(snap, fingerprint)
		}
		return gameLobbyResultMsg{snapshot: snap, kind: lobbyActionJoin, err: err}
	}
}

func joinRandomGameCmd(fingerprint string, tc managers.TimeControl) tea.Cmd {
	return func() tea.Msg {
		snap, err := gameManager.JoinRandomGame(fingerprint, tc)
		if err == nil && snap != nil {
			notifyOpponentJoined(snap, fingerprint)
		}
		return gameLobbyResultMsg{snapshot: snap, kind: lobbyActionJoinRandom, err: err}
	}
}

// applyMoveCmd runs the manager-side move under the manager mutex, then
// fans out the resulting snapshot to the opponent (if any) before
// returning the local moveAppliedMsg. All of this happens in the cmd
// goroutine — the bubbletea event loop only ever sees a single
// moveAppliedMsg landing in Update().
func applyMoveCmd(fingerprint, move string) tea.Cmd {
	return func() tea.Msg {
		snap, err := gameManager.MakeMove(fingerprint, move)
		if err == nil && snap != nil {
			notifyOpponentMoved(snap, fingerprint, move)
		}
		return moveAppliedMsg{snapshot: snap, move: move, err: err}
	}
}

// applyMouseMoveCmd is the mouse-click variant. The chess library
// requires an explicit promotion suffix ("e7e8q"), but a click on e7 then
// e8 only gives us "e7e8". To preserve the pre-rewrite UX of auto-queen-
// on-click, we first try the bare UCI string; on failure we retry with
// "q" appended. If the retry succeeds we report the trailing-q form as
// the move that was played; if both fail we return the original error.
func applyMouseMoveCmd(fingerprint, move string) tea.Cmd {
	return func() tea.Msg {
		snap, err := gameManager.MakeMove(fingerprint, move)
		if err != nil {
			if snap2, err2 := gameManager.MakeMove(fingerprint, move+"q"); err2 == nil {
				snap = snap2
				move += "q"
				err = nil
			}
		}
		if err == nil && snap != nil {
			notifyOpponentMoved(snap, fingerprint, move)
		}
		return moveAppliedMsg{snapshot: snap, move: move, err: err}
	}
}

// notifyOpponentJoined sends the freshly-built snapshot to the other
// player's program so they see the game transition from waiting to
// in-progress without having to round-trip through the manager themselves.
// Called from inside cmd goroutines, never from Update().
func notifyOpponentJoined(snap *managers.Snapshot, joinerFingerprint string) {
	oppFP := snap.OpponentFingerprint(joinerFingerprint)
	if oppFP == "" {
		return
	}
	prog := sessionManager.GetProgram(oppFP)
	if prog == nil {
		return
	}
	prog.Send(opponentJoinedGameMsg{
		opponentName: gameManager.PlayerUsername(joinerFingerprint),
		snapshot:     snap,
	})
}

// notifyOpponentMoved sends the freshly-built post-move snapshot to the
// opponent. The receiver replaces their snapshot wholesale, so the
// opponent's board re-renders correctly without re-reading manager state.
func notifyOpponentMoved(snap *managers.Snapshot, moverFingerprint string, move string) {
	oppFP := snap.OpponentFingerprint(moverFingerprint)
	if oppFP == "" {
		return
	}
	prog := sessionManager.GetProgram(oppFP)
	if prog == nil {
		return
	}
	prog.Send(gameUpdatedMsg{move: move, snapshot: snap})
}

// persistAndRemoveGameCmd persists a finished game and clears it from
// memory off the Bubble Tea event loop. The DB write happens in a
// goroutine so Update() doesn't block waiting on Postgres. Returns a nil
// msg because each affected player is notified via
// prog.Send(gamesRefreshMsg{}) inside the goroutine (the fanout reaches
// both players, not just the caller).
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

// --- Helpers used by both Update and View -----------------------------------

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
	fen := m.snapshot.FEN
	colorIsWhite := m.snapshot.PlayerColor(m.ctx.fingerPrint) != chess.Black
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

func gameTurnLine(s *managers.Snapshot, fingerprint string) string {
	if s == nil {
		return ""
	}
	color := s.PlayerColor(fingerprint)
	if color == chess.NoColor {
		return ""
	}
	colorName := strings.ToLower(color.Name())
	if s.Status != managers.GameStatusInProgress {
		return "You are " + colorName + "."
	}
	if s.IsPlayersTurn(fingerprint) {
		return "You are " + colorName + ". Your turn."
	}
	return "You are " + colorName + ". " + s.Turn.Name() + " to move."
}

// --- Update -----------------------------------------------------------------

// Update is the entry point for the game page. It first handles
// page-scoped messages (opponent joined / opponent moved, clock ticks,
// time forfeit, move-applied, lobby-result) and then routes to a
// state-specific handler based on whether the player has a game and what
// state that game is in. Each sub-handler can assume the invariants for
// its state, so we don't have to defensively re-check them.
func (m gameModel) Update(msg tea.Msg) (gameModel, tea.Cmd) {
	switch msg := msg.(type) {
	case opponentJoinedGameMsg:
		if msg.snapshot != nil {
			m = m.withSnapshot(msg.snapshot)
		}
		opponent := msg.opponentName
		if opponent == "" {
			opponent = "Opponent"
		}
		m.gameNotice = opponent + " joined. Game on."
		if m.snapshot != nil && m.snapshot.Status == managers.GameStatusInProgress {
			return m, m.moveInput.Focus()
		}
		return m, nil
	case gameUpdatedMsg:
		if msg.snapshot != nil {
			m = m.withSnapshot(msg.snapshot)
		}
		if msg.move != "" {
			m.gameNotice = "Opponent played " + msg.move + "."
		}
		m.selected = ""
		m.possibleMoves = nil
		if m.snapshot != nil && m.snapshot.Status == managers.GameStatusFinished {
			return m, gamesRefreshCmd()
		}
		return m, nil
	case ClockUpdateMsg:
		if m.snapshot != nil && m.snapshot.ID == msg.GameID {
			m.whiteTimeLeft = msg.WhiteTime
			m.blackTimeLeft = msg.BlackTime
		}
		return m, nil
	case TimeForfeitMsg:
		if m.snapshot == nil || m.snapshot.ID != msg.GameID {
			return m, nil
		}
		if msg.Snapshot != nil {
			m = m.withSnapshot(msg.Snapshot)
		}
		m.movePending = false
		m.selected = ""
		m.possibleMoves = nil
		if msg.LoserColor == chess.White {
			m.gameNotice = "White ran out of time. Black wins!"
		} else {
			m.gameNotice = "Black ran out of time. White wins!"
		}
		return m, gamesRefreshCmd()
	case moveAppliedMsg:
		return m.handleMoveApplied(msg)
	case gameLobbyResultMsg:
		return m.handleLobbyResult(msg)
	}

	if m.snapshot == nil {
		return m.updateGameLobby(msg)
	}
	switch m.snapshot.Status {
	case managers.GameStatusWaiting:
		return m.updateGameWaiting(msg)
	case managers.GameStatusInProgress:
		return m.updateGameInProgress(msg)
	case managers.GameStatusFinished:
		return m.updateGameFinished(msg)
	}
	return m, nil
}

func (m gameModel) handleLobbyResult(msg gameLobbyResultMsg) (gameModel, tea.Cmd) {
	if msg.err != nil {
		m.gameNotice = msg.err.Error()
		return m, nil
	}
	if msg.snapshot == nil {
		return m, nil
	}
	m = m.withSnapshot(msg.snapshot)
	m.gameJoinInput.SetValue("")
	m.moveInput.SetValue("")
	switch msg.kind {
	case lobbyActionCreate:
		m.gameNotice = "Created " + msg.snapshot.TimeControl.String() + " game " + msg.snapshot.ID + ". Share the ID with your opponent."
		return m, nil
	case lobbyActionJoin, lobbyActionJoinRandom:
		m.gameNotice = "Joined " + msg.snapshot.TimeControl.String() + " game " + msg.snapshot.ID + "."
		if msg.snapshot.Status == managers.GameStatusInProgress {
			return m, m.moveInput.Focus()
		}
		return m, nil
	}
	return m, nil
}

// handleMoveApplied is the convergence point for both keyboard-submitted
// moves and mouse-submitted moves. The cmd has already pushed the move
// through the manager and notified the opponent; here we just project the
// new snapshot into the model and fire any follow-up cmd (game-end
// persistence).
func (m gameModel) handleMoveApplied(msg moveAppliedMsg) (gameModel, tea.Cmd) {
	m.movePending = false
	if msg.err != nil {
		m.gameNotice = "Move rejected: " + msg.err.Error()
		return m, nil
	}
	if msg.snapshot == nil {
		return m, nil
	}
	m = m.withSnapshot(msg.snapshot)
	m.moveInput.SetValue("")
	m.gameNotice = "Played " + msg.move + "."
	m.selected = ""
	m.possibleMoves = nil

	if msg.snapshot.Status == managers.GameStatusFinished {
		return m, persistAndRemoveGameCmd(msg.snapshot.ID)
	}
	return m, nil
}

// updateGameLobby handles input when the player is on the game page but
// not yet in a game: pick a time control, then create, join random, or
// join by ID. All three lobby actions are async cmds — they take the
// gameManager mutex briefly, which is in-memory and fast, but routing
// through a cmd uniformly keeps Update() free of state mutations.
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
			return m, createGameCmd(m.ctx.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
		case "ctrl+r":
			if m.selectedTimeControl == NoTimeControl {
				m.gameNotice = "Select a time control first (1, 3, or 5)."
				return m, nil
			}
			return m, joinRandomGameCmd(m.ctx.fingerPrint, m.selectedTimeControl.ToGameTimeControl())
		case "enter":
			gameID := strings.TrimSpace(m.gameJoinInput.Value())
			if gameID == "" {
				return m, nil
			}
			return m, joinGameCmd(m.ctx.fingerPrint, gameID)
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
// movePending acts as UI-level backpressure: a second submission while
// the first is still in flight is dropped so we never queue two moves.
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
			if m.movePending {
				return m, nil
			}
			move := strings.ToLower(strings.TrimSpace(m.moveInput.Value()))
			if move == "" {
				return m, nil
			}
			m.movePending = true
			return m, applyMoveCmd(m.ctx.fingerPrint, move)
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

	if m.movePending {
		return m, nil
	}

	if !m.snapshot.IsPlayersTurn(m.ctx.fingerPrint) {
		m.selected = ""
		m.possibleMoves = nil
		return m, nil
	}

	colorIsWhite := m.snapshot.PlayerColor(m.ctx.fingerPrint) == chess.White
	doesClick := false

	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			pos := convertToChessboardPosition(j, i, colorIsWhite)
			if !m.ctx.zone.Get(pos).InBounds(msg) {
				continue
			}
			doesClick = true

			if m.selected == "" {
				m.selected = pos
				m.possibleMoves = legalMovesFromSquare(m.snapshot.FEN, pos)
				return m, nil
			}

			moveUCI := m.selected + pos
			m.selected = ""
			m.possibleMoves = nil
			m.movePending = true
			return m, applyMouseMoveCmd(m.ctx.fingerPrint, moveUCI)
		}
	}

	if !doesClick {
		m.selected = ""
		m.possibleMoves = nil
	}
	return m, nil
}

// legalMovesFromSquare returns the destination squares a piece on `from`
// can move to. The chess library only accepts UCI strings, so we parse
// the current FEN into a transient game, enumerate moves, and filter by
// the origin square. Promotion targets are normalized back to the bare
// destination (e.g. "e7e8q" -> "e8") so the highlight matches what the
// player would click; applyMouseMoveCmd then re-adds the "q" suffix when
// the move is submitted.
func legalMovesFromSquare(fen, from string) []string {
	fenOpt, err := chess.FEN(fen)
	if err != nil {
		return nil
	}
	g := chess.NewGame(chess.UseNotation(chess.UCINotation{}), fenOpt)
	out := []string{}
	for _, move := range g.ValidMoves() {
		moveStr := move.String()
		if len(moveStr) < 4 {
			continue
		}
		if moveStr[:2] != from {
			continue
		}
		dest := moveStr[2:]
		if len(dest) == 3 {
			dest = dest[:2]
		}
		out = append(out, dest)
	}
	return out
}

// updateGameFinished handles input after the game ended.
func (m gameModel) updateGameFinished(msg tea.Msg) (gameModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, navigateToChatCmdGame()
	}
	return m, nil
}

// --- View -------------------------------------------------------------------

func (m gameModel) getGameBoard() string {
	if m.snapshot != nil && m.snapshot.FEN != "" {
		return m.renderBoardFromFEN()
	}
	return gameNoGame
}

func (m gameModel) View() string {
	if m.snapshot == nil {
		return m.viewGameLobby()
	}
	switch m.snapshot.Status {
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
		infoStyle.Render("Game ID: ") + highlightStyle.Render(m.snapshot.ID),
		infoStyle.Render(gameStatusLine(m.snapshot.Status)),
	}
	if m.snapshot.TimeControl != 0 {
		rows = append(rows, infoStyle.Render("Time control: ")+highlightStyle.Render(m.snapshot.TimeControl.String()))
	}
	if turnLine := gameTurnLine(m.snapshot, m.ctx.fingerPrint); turnLine != "" {
		rows = append(rows, highlightStyle.Render(turnLine))
	}
	if m.snapshot.Status == managers.GameStatusInProgress {
		playerColor := m.snapshot.PlayerColor(m.ctx.fingerPrint)
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
