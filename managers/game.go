package managers

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/google/uuid"
	"github.com/notnil/chess"
)

const (
	GameStatusWaiting    = "waiting"
	GameStatusInProgress = "in progress"
	GameStatusFinished   = "finished"
)

const (
	MethodTimeForfeit = "time forfeit"
)

type TimeControl int

const (
	TimeControl1Min TimeControl = 1
	TimeControl3Min TimeControl = 3
	TimeControl5Min TimeControl = 5
)

func (tc TimeControl) Duration() time.Duration {
	return time.Duration(tc) * time.Minute
}

func (tc TimeControl) String() string {
	return fmt.Sprintf("%d min", tc)
}

type GamePlayer struct {
	fingerprint   string
	username      string
	currentGameId string
}

// Game is the authoritative in-memory state. It is owned by GameManager and
// only ever mutated while holding gm.mu. Callers outside the package see
// Snapshot values instead of *Game pointers, so they cannot observe partial
// mutations or race with the manager's writers.
type Game struct {
	id          string
	whitePlayer *GamePlayer
	blackPlayer *GamePlayer
	status      string
	game        *chess.Game

	timeControl   TimeControl
	whiteTimeLeft time.Duration
	blackTimeLeft time.Duration
	turnStartedAt time.Time
}

// Snapshot is an immutable, perspective-independent view of a Game taken at
// a point in time. It is the only thing that crosses the GameManager
// package boundary — TUI models and the clock ticker store snapshots and
// render from them, so no goroutine outside the manager ever reads a *Game
// directly. The few perspective-dependent helpers (PlayerColor,
// IsPlayersTurn) take a fingerprint and derive their answer from the
// snapshot's own fields.
type Snapshot struct {
	ID               string
	FEN              string
	PGN              string
	Status           string
	Turn             chess.Color
	Outcome          string
	Method           string
	TimeControl      TimeControl
	WhiteTimeLeft    time.Duration
	BlackTimeLeft    time.Duration
	WhiteFingerprint string
	BlackFingerprint string
	WhiteUsername    string
	BlackUsername    string
}

func (s *Snapshot) PlayerColor(fingerprint string) chess.Color {
	if s == nil {
		return chess.NoColor
	}
	if fingerprint != "" && s.WhiteFingerprint == fingerprint {
		return chess.White
	}
	if fingerprint != "" && s.BlackFingerprint == fingerprint {
		return chess.Black
	}
	return chess.NoColor
}

func (s *Snapshot) IsPlayersTurn(fingerprint string) bool {
	color := s.PlayerColor(fingerprint)
	if color == chess.NoColor {
		return false
	}
	return s.Turn == color
}

func (s *Snapshot) OpponentFingerprint(fingerprint string) string {
	if s == nil {
		return ""
	}
	if s.WhiteFingerprint != "" && s.WhiteFingerprint != fingerprint {
		return s.WhiteFingerprint
	}
	if s.BlackFingerprint != "" && s.BlackFingerprint != fingerprint {
		return s.BlackFingerprint
	}
	return ""
}

// snapshot copies the current state of a Game into a Snapshot. Must be
// called with gm.mu held.
func (g *Game) snapshot() *Snapshot {
	if g == nil {
		return nil
	}
	whiteTime, blackTime := g.currentClocksLocked()
	s := &Snapshot{
		ID:            g.id,
		Status:        g.status,
		TimeControl:   g.timeControl,
		WhiteTimeLeft: whiteTime,
		BlackTimeLeft: blackTime,
	}
	if g.whitePlayer != nil {
		s.WhiteFingerprint = g.whitePlayer.fingerprint
		s.WhiteUsername = g.whitePlayer.username
	}
	if g.blackPlayer != nil {
		s.BlackFingerprint = g.blackPlayer.fingerprint
		s.BlackUsername = g.blackPlayer.username
	}
	if g.game != nil {
		s.FEN = g.game.FEN()
		s.PGN = g.game.String()
		s.Outcome = string(g.game.Outcome())
		s.Method = g.game.Method().String()
		if pos := g.game.Position(); pos != nil {
			s.Turn = pos.Turn()
		}
	}
	return s
}

// currentClocksLocked computes the live clock values, deducting elapsed
// time since the current turn started if the game is in progress. Callers
// must hold gm.mu.
func (g *Game) currentClocksLocked() (time.Duration, time.Duration) {
	whiteTime := g.whiteTimeLeft
	blackTime := g.blackTimeLeft
	if g.status != GameStatusInProgress || g.turnStartedAt.IsZero() {
		return whiteTime, blackTime
	}
	elapsed := time.Since(g.turnStartedAt)
	turn := chess.NoColor
	if g.game != nil && g.game.Position() != nil {
		turn = g.game.Position().Turn()
	}
	if turn == chess.White {
		whiteTime -= elapsed
	} else {
		blackTime -= elapsed
	}
	return whiteTime, blackTime
}

type GameManager struct {
	mu      sync.Mutex
	games   map[string]*Game
	players map[string]*GamePlayer
}

func NewGameManager() *GameManager {
	return &GameManager{
		games:   make(map[string]*Game),
		players: make(map[string]*GamePlayer),
	}
}

func (gm *GameManager) SetPlayer(fingerprint string, username string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	if player, exists := gm.players[fingerprint]; exists {
		if username != "" {
			player.username = username
		}
		return
	}

	gm.players[fingerprint] = &GamePlayer{
		fingerprint: fingerprint,
		username:    username,
	}
}

func (gm *GameManager) getColor() chess.Color {
	randomColor := rand.Intn(2) == 0
	if randomColor {
		return chess.White
	}
	return chess.Black
}

func (gm *GameManager) CreateGame(fingerprint string, tc TimeControl) (*Snapshot, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return nil, fmt.Errorf("player already in a game")
	}

	gameId := uuid.New().String()
	player.currentGameId = gameId
	color := gm.getColor()
	g := &Game{
		id:            gameId,
		status:        GameStatusWaiting,
		game:          chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		timeControl:   tc,
		whiteTimeLeft: tc.Duration(),
		blackTimeLeft: tc.Duration(),
	}
	if color == chess.White {
		g.whitePlayer = player
	} else {
		g.blackPlayer = player
	}
	gm.games[gameId] = g

	return g.snapshot(), nil
}

func (gm *GameManager) JoinRandomGame(fingerprint string, tc TimeControl) (*Snapshot, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return nil, fmt.Errorf("player already in a game")
	}

	var halfGame *Game
	for _, game := range gm.games {
		if game.status != GameStatusWaiting {
			continue
		}
		if game.timeControl != tc {
			continue
		}
		if game.whitePlayer != nil && game.whitePlayer.fingerprint == fingerprint {
			continue
		}
		if game.blackPlayer != nil && game.blackPlayer.fingerprint == fingerprint {
			continue
		}
		halfGame = game
		break
	}
	if halfGame == nil {
		return nil, fmt.Errorf("no half game found")
	}

	if halfGame.whitePlayer == nil {
		halfGame.whitePlayer = player
	} else {
		halfGame.blackPlayer = player
	}

	halfGame.status = GameStatusInProgress
	halfGame.turnStartedAt = time.Now()
	player.currentGameId = halfGame.id

	return halfGame.snapshot(), nil
}

func (gm *GameManager) JoinGame(fingerprint string, gameId string) (*Snapshot, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return nil, fmt.Errorf("player already in a game")
	}

	game := gm.games[gameId]
	if game == nil {
		return nil, fmt.Errorf("game not found")
	}
	if game.status != GameStatusWaiting {
		return nil, fmt.Errorf("game is not available")
	}
	if game.whitePlayer != nil && game.blackPlayer != nil {
		return nil, fmt.Errorf("game is already full")
	}
	if game.whitePlayer != nil && game.whitePlayer.fingerprint == fingerprint {
		return nil, fmt.Errorf("cannot join your own game")
	}
	if game.blackPlayer != nil && game.blackPlayer.fingerprint == fingerprint {
		return nil, fmt.Errorf("cannot join your own game")
	}

	if game.whitePlayer == nil {
		game.whitePlayer = player
	} else {
		game.blackPlayer = player
	}

	game.status = GameStatusInProgress
	game.turnStartedAt = time.Now()
	player.currentGameId = gameId

	return game.snapshot(), nil
}

func (gm *GameManager) MakeMove(fingerprint string, move string) (*Snapshot, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return nil, fmt.Errorf("player not found")
	}
	if player.currentGameId == "" {
		return nil, fmt.Errorf("join a game first")
	}

	game := gm.games[player.currentGameId]
	if game == nil {
		return nil, fmt.Errorf("game not found")
	}
	if game.status == GameStatusWaiting {
		return nil, fmt.Errorf("wait for an opponent to join")
	}
	if game.status == GameStatusFinished {
		return nil, fmt.Errorf("this game is already finished")
	}

	playerColor := chess.NoColor
	if game.whitePlayer != nil && game.whitePlayer.fingerprint == fingerprint {
		playerColor = chess.White
	} else if game.blackPlayer != nil && game.blackPlayer.fingerprint == fingerprint {
		playerColor = chess.Black
	}
	if playerColor == chess.NoColor {
		return nil, fmt.Errorf("not a player in this game")
	}

	turn := chess.NoColor
	if pos := game.game.Position(); pos != nil {
		turn = pos.Turn()
	}
	if turn != playerColor {
		return nil, fmt.Errorf("wait for your turn")
	}

	whiteTime, blackTime := game.currentClocksLocked()
	if turn == chess.White {
		if whiteTime <= 0 {
			return nil, fmt.Errorf("time ran out")
		}
	} else if blackTime <= 0 {
		return nil, fmt.Errorf("time ran out")
	}

	if err := game.game.MoveStr(move); err != nil {
		return nil, err
	}

	elapsed := time.Since(game.turnStartedAt)
	if playerColor == chess.White {
		game.whiteTimeLeft -= elapsed
		if game.whiteTimeLeft < 0 {
			game.whiteTimeLeft = 0
		}
	} else {
		game.blackTimeLeft -= elapsed
		if game.blackTimeLeft < 0 {
			game.blackTimeLeft = 0
		}
	}

	if game.game.Outcome() != chess.NoOutcome {
		game.status = GameStatusFinished
	} else {
		game.status = GameStatusInProgress
		game.turnStartedAt = time.Now()
	}

	return game.snapshot(), nil
}

// EndByTimeForfeit ends the named game by time forfeit and returns a
// snapshot of the now-finished game. Returns nil if the game does not
// exist or is no longer in progress (e.g. concurrently finished by a
// regular move) so callers can distinguish "I caused the forfeit" from
// "it ended before I got there".
func (gm *GameManager) EndByTimeForfeit(gameID string, loser chess.Color) *Snapshot {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	game := gm.games[gameID]
	if game == nil || game.status != GameStatusInProgress {
		return nil
	}

	elapsed := time.Since(game.turnStartedAt)
	turn := chess.NoColor
	if pos := game.game.Position(); pos != nil {
		turn = pos.Turn()
	}
	if turn == chess.White {
		game.whiteTimeLeft -= elapsed
		if game.whiteTimeLeft < 0 {
			game.whiteTimeLeft = 0
		}
	} else {
		game.blackTimeLeft -= elapsed
		if game.blackTimeLeft < 0 {
			game.blackTimeLeft = 0
		}
	}

	game.status = GameStatusFinished
	game.game.Resign(loser)
	return game.snapshot()
}

func (gm *GameManager) PlayerUsername(fingerprint string) string {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return ""
	}
	return player.username
}

// SnapshotForPlayer returns the current snapshot of the game the player is
// in, or nil if they are not currently in a game.
func (gm *GameManager) SnapshotForPlayer(fingerprint string) *Snapshot {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil || player.currentGameId == "" {
		return nil
	}
	g := gm.games[player.currentGameId]
	if g == nil {
		return nil
	}
	return g.snapshot()
}

// SnapshotByID returns a snapshot by game id. Useful for sending the same
// game state to multiple programs (e.g. notifying both players).
func (gm *GameManager) SnapshotByID(gameID string) *Snapshot {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	g := gm.games[gameID]
	if g == nil {
		return nil
	}
	return g.snapshot()
}

func (gm *GameManager) BuildGameRecord(gameID string) common.Game {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	game := gm.games[gameID]
	return common.Game{
		GameID:           game.id,
		WhiteFingerprint: game.whitePlayer.fingerprint,
		WhiteUsername:    game.whitePlayer.username,
		BlackFingerprint: game.blackPlayer.fingerprint,
		BlackUsername:    game.blackPlayer.username,
		PGN:              game.game.String(),
		Outcome:          string(game.game.Outcome()),
		Method:           game.game.Method().String(),
		TimeControl:      int(game.timeControl),
	}
}

// RemoveGame drops a game from the in-memory map and clears the
// currentGameId on both players so the Game object can be GC'd. Must only
// be called after the game has been durably persisted.
func (gm *GameManager) RemoveGame(gameID string) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	game := gm.games[gameID]
	if game == nil {
		return
	}
	if game.whitePlayer != nil {
		game.whitePlayer.currentGameId = ""
	}
	if game.blackPlayer != nil {
		game.blackPlayer.currentGameId = ""
	}
	delete(gm.games, gameID)
}

func (gm *GameManager) OpponentFingerprint(gameID string, fingerprint string) string {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	g := gm.games[gameID]
	if g == nil {
		return ""
	}
	if g.whitePlayer != nil && g.whitePlayer.fingerprint != fingerprint {
		return g.whitePlayer.fingerprint
	}
	if g.blackPlayer != nil && g.blackPlayer.fingerprint != fingerprint {
		return g.blackPlayer.fingerprint
	}
	return ""
}

// TickInfo is the per-tick view of an in-progress game used by the clock
// ticker. Holding gm.mu while building these allows the ticker to act on
// them without re-entering the manager for each field read (and without
// racing against MakeMove).
type TickInfo struct {
	GameID           string
	WhiteTime        time.Duration
	BlackTime        time.Duration
	Expired          bool
	LoserColor       chess.Color
	WhiteFingerprint string
	BlackFingerprint string
}

// TickAll returns a TickInfo for every in-progress game, computing
// up-to-date clocks and expiry under the manager lock. The ticker walks
// the result without further coordination.
func (gm *GameManager) TickAll() []TickInfo {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	var out []TickInfo
	for _, game := range gm.games {
		if game.status != GameStatusInProgress {
			continue
		}
		whiteTime, blackTime := game.currentClocksLocked()
		info := TickInfo{
			GameID:    game.id,
			WhiteTime: whiteTime,
			BlackTime: blackTime,
		}
		if game.whitePlayer != nil {
			info.WhiteFingerprint = game.whitePlayer.fingerprint
		}
		if game.blackPlayer != nil {
			info.BlackFingerprint = game.blackPlayer.fingerprint
		}
		if whiteTime <= 0 {
			info.Expired = true
			info.LoserColor = chess.White
		} else if blackTime <= 0 {
			info.Expired = true
			info.LoserColor = chess.Black
		}
		out = append(out, info)
	}
	return out
}
