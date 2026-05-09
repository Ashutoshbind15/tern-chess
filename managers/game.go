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

func (g *Game) ID() string {
	if g == nil {
		return ""
	}
	return g.id
}

func (g *Game) Status() string {
	if g == nil {
		return ""
	}
	return g.status
}

func (g *Game) Game() *chess.Game {
	if g == nil {
		return nil
	}
	return g.game
}

func (g *Game) TimeControl() TimeControl {
	if g == nil {
		return 0
	}
	return g.timeControl
}

func (g *Game) PlayerColor(fingerprint string) chess.Color {
	if g == nil {
		return chess.NoColor
	}
	if g.whitePlayer != nil && g.whitePlayer.fingerprint == fingerprint {
		return chess.White
	}
	if g.blackPlayer != nil && g.blackPlayer.fingerprint == fingerprint {
		return chess.Black
	}
	return chess.NoColor
}

func (g *Game) Turn() chess.Color {
	if g == nil || g.game == nil || g.game.Position() == nil {
		return chess.NoColor
	}
	return g.game.Position().Turn()
}

func (g *Game) IsPlayersTurn(fingerprint string) bool {
	playerColor := g.PlayerColor(fingerprint)
	if playerColor == chess.NoColor {
		return false
	}
	return g.Turn() == playerColor
}

func (g *Game) CurrentClocks() (whiteTime time.Duration, blackTime time.Duration) {
	if g == nil {
		return 0, 0
	}
	whiteTime = g.whiteTimeLeft
	blackTime = g.blackTimeLeft

	if g.status == GameStatusInProgress && !g.turnStartedAt.IsZero() {
		elapsed := time.Since(g.turnStartedAt)
		if g.Turn() == chess.White {
			whiteTime -= elapsed
		} else {
			blackTime -= elapsed
		}
	}
	return whiteTime, blackTime
}

func (g *Game) IsTimeExpired() (bool, chess.Color) {
	if g == nil || g.status != GameStatusInProgress {
		return false, chess.NoColor
	}
	whiteTime, blackTime := g.CurrentClocks()
	if whiteTime <= 0 {
		return true, chess.White
	}
	if blackTime <= 0 {
		return true, chess.Black
	}
	return false, chess.NoColor
}

type GameManager struct {
	mu     sync.Mutex
	games  map[string]*Game
	players map[string]*GamePlayer
}

func NewGameManager() *GameManager {
	return &GameManager{
		games:   make(map[string]*Game),
		players: make(map[string]*GamePlayer),
	}
}

func (gm *GameManager) SetPlayer(fingerprint string, username string) {

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

func (gm *GameManager) GetIdlePlayer() *GamePlayer {
	for _, player := range gm.players {
		if player.currentGameId == "" {
			return player
		}
	}
	return nil
}

func (gm *GameManager) getColor() chess.Color {
	randomColor := rand.Intn(2) == 0
	if randomColor {
		return chess.White
	}
	return chess.Black
}

func (gm *GameManager) CreateGame(fingerprint string, tc TimeControl) (string, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return "", fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return "", fmt.Errorf("player already in a game")
	}

	gameId := uuid.New().String()
	player.currentGameId = gameId
	color := gm.getColor()
	if color == chess.White {
		gm.games[gameId] = &Game{
			id:            gameId,
			whitePlayer:   player,
			blackPlayer:   nil,
			status:        GameStatusWaiting,
			game:          chess.NewGame(chess.UseNotation(chess.UCINotation{})),
			timeControl:   tc,
			whiteTimeLeft: tc.Duration(),
			blackTimeLeft: tc.Duration(),
		}
	} else {
		gm.games[gameId] = &Game{
			id:            gameId,
			whitePlayer:   nil,
			blackPlayer:   player,
			status:        GameStatusWaiting,
			game:          chess.NewGame(chess.UseNotation(chess.UCINotation{})),
			timeControl:   tc,
			whiteTimeLeft: tc.Duration(),
			blackTimeLeft: tc.Duration(),
		}
	}

	return gameId, nil
}

func (gm *GameManager) GetHalfGame(tc TimeControl) *Game {
	for _, game := range gm.games {
		if game.status == GameStatusWaiting && game.timeControl == tc {
			return game
		}
	}
	return nil
}

func (gm *GameManager) JoinRandomGame(fingerprint string, tc TimeControl) (string, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return "", fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return "", fmt.Errorf("player already in a game")
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
		return "", fmt.Errorf("no half game found")
	}

	if halfGame.whitePlayer == nil {
		halfGame.whitePlayer = player
	} else {
		halfGame.blackPlayer = player
	}

	halfGame.status = GameStatusInProgress
	halfGame.turnStartedAt = time.Now()
	player.currentGameId = halfGame.id

	return halfGame.id, nil
}

func (gm *GameManager) JoinGame(fingerprint string, gameId string) (string, error) {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil {
		return "", fmt.Errorf("player not found")
	}
	if player.currentGameId != "" {
		return "", fmt.Errorf("player already in a game")
	}

	game := gm.games[gameId]
	if game == nil {
		return "", fmt.Errorf("game not found")
	}
	if game.status != GameStatusWaiting {
		return "", fmt.Errorf("game is not available")
	}
	if game.whitePlayer != nil && game.blackPlayer != nil {
		return "", fmt.Errorf("game is already full")
	}
	if game.whitePlayer != nil && game.whitePlayer.fingerprint == fingerprint {
		return "", fmt.Errorf("cannot join your own game")
	}
	if game.blackPlayer != nil && game.blackPlayer.fingerprint == fingerprint {
		return "", fmt.Errorf("cannot join your own game")
	}

	if game.whitePlayer == nil {
		game.whitePlayer = player
	} else {
		game.blackPlayer = player
	}

	game.status = GameStatusInProgress
	game.turnStartedAt = time.Now()
	player.currentGameId = gameId

	return gameId, nil
}

func (gm *GameManager) MakeMove(fingerprint string, move string) (*Game, error) {
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

	if !game.IsPlayersTurn(fingerprint) {
		return nil, fmt.Errorf("wait for your turn")
	}

	whiteTime, blackTime := game.CurrentClocks()
	if game.Turn() == chess.White {
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
	playerColor := game.PlayerColor(fingerprint)
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

	return game, nil
}

func (gm *GameManager) EndByTimeForfeit(gameID string, loser chess.Color) *Game {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	game := gm.games[gameID]
	if game == nil || game.status != GameStatusInProgress {
		return nil
	}

	elapsed := time.Since(game.turnStartedAt)
	if game.Turn() == chess.White {
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
	return game
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

func (gm *GameManager) GameForPlayer(fingerprint string) *Game {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	player := gm.players[fingerprint]
	if player == nil || player.currentGameId == "" {
		return nil
	}
	return gm.games[player.currentGameId]
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
		PGN:             game.game.String(),
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
	game.whitePlayer.currentGameId = ""
	game.blackPlayer.currentGameId = ""
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

func (gm *GameManager) AllInProgressGames() []*Game {
	gm.mu.Lock()
	defer gm.mu.Unlock()

	var result []*Game
	for _, game := range gm.games {
		if game.status == GameStatusInProgress {
			result = append(result, game)
		}
	}
	return result
}