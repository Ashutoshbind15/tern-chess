package managers

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"github.com/google/uuid"
	"github.com/notnil/chess"
)

// BotGame is intentionally thinner than Game. There is exactly one human
// player, untimed, played to victory. The bot has no fingerprint and is not
// tracked as a player anywhere; it lives only as the side that the engine
// service plays for.
type BotGame struct {
	id          string
	fingerprint string
	username    string
	playerColor chess.Color
	botLevel    int
	game        *chess.Game
	status      string
}

func (g *BotGame) ID() string {
	if g == nil {
		return ""
	}
	return g.id
}

func (g *BotGame) Status() string {
	if g == nil {
		return ""
	}
	return g.status
}

func (g *BotGame) Game() *chess.Game {
	if g == nil {
		return nil
	}
	return g.game
}

func (g *BotGame) PlayerColor() chess.Color {
	if g == nil {
		return chess.NoColor
	}
	return g.playerColor
}

func (g *BotGame) BotColor() chess.Color {
	if g == nil {
		return chess.NoColor
	}
	if g.playerColor == chess.White {
		return chess.Black
	}
	return chess.White
}

func (g *BotGame) BotLevel() int {
	if g == nil {
		return 0
	}
	return g.botLevel
}

func (g *BotGame) Turn() chess.Color {
	if g == nil || g.game == nil || g.game.Position() == nil {
		return chess.NoColor
	}
	return g.game.Position().Turn()
}

func (g *BotGame) IsPlayersTurn() bool {
	if g == nil {
		return false
	}
	return g.Turn() == g.playerColor
}

func (g *BotGame) FEN() string {
	if g == nil || g.game == nil {
		return ""
	}
	return g.game.FEN()
}

// BotGameManager owns the lifecycle of bot games. It does not coordinate
// with the multiplayer GameManager; a player can in principle have a bot
// game and a multiplayer game at the same time, but in practice we only
// surface one at a time in the UI.
type BotGameManager struct {
	mu          sync.Mutex
	games       map[string]*BotGame
	playerGames map[string]string // fingerprint -> bot game id
}

func NewBotGameManager() *BotGameManager {
	return &BotGameManager{
		games:       make(map[string]*BotGame),
		playerGames: make(map[string]string),
	}
}

func clampBotLevel(level int) int {
	if level < BotMinLevel {
		return BotMinLevel
	}
	if level > BotMaxLevel {
		return BotMaxLevel
	}
	return level
}

// CreateBotGame starts a new untimed game for the given player. If
// playerColor is chess.NoColor a side is picked at random.
func (bm *BotGameManager) CreateBotGame(fingerprint, username string, playerColor chess.Color, level int) (*BotGame, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	if fingerprint == "" {
		return nil, fmt.Errorf("missing fingerprint")
	}
	if existing, ok := bm.playerGames[fingerprint]; ok && existing != "" {
		return nil, fmt.Errorf("you already have a bot game in progress")
	}

	if playerColor == chess.NoColor {
		if rand.Intn(2) == 0 {
			playerColor = chess.White
		} else {
			playerColor = chess.Black
		}
	}

	id := uuid.New().String()
	game := &BotGame{
		id:          id,
		fingerprint: fingerprint,
		username:    username,
		playerColor: playerColor,
		botLevel:    clampBotLevel(level),
		game:        chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		status:      GameStatusInProgress,
	}
	bm.games[id] = game
	bm.playerGames[fingerprint] = id
	return game, nil
}

// GameForPlayer returns the active bot game for a given player or nil.
func (bm *BotGameManager) GameForPlayer(fingerprint string) *BotGame {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	id := bm.playerGames[fingerprint]
	if id == "" {
		return nil
	}
	return bm.games[id]
}

// MakePlayerMove validates and applies a player move. Returns the updated
// game, or an error if the move is illegal / not the player's turn.
func (bm *BotGameManager) MakePlayerMove(fingerprint, move string) (*BotGame, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	game := bm.gameForLocked(fingerprint)
	if game == nil {
		return nil, fmt.Errorf("no bot game in progress")
	}
	if game.status != GameStatusInProgress {
		return nil, fmt.Errorf("game is not in progress")
	}
	if !game.IsPlayersTurn() {
		return nil, fmt.Errorf("wait for the bot to move")
	}

	if err := game.game.MoveStr(strings.TrimSpace(move)); err != nil {
		return nil, err
	}
	if game.game.Outcome() != chess.NoOutcome {
		game.status = GameStatusFinished
	}
	return game, nil
}

// ApplyBotMove applies the move returned by the engine service. Bot moves
// arrive over the network so we apply them by id rather than by player.
func (bm *BotGameManager) ApplyBotMove(gameID, move string) (*BotGame, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	game := bm.games[gameID]
	if game == nil {
		return nil, fmt.Errorf("bot game not found")
	}
	if game.status != GameStatusInProgress {
		return nil, fmt.Errorf("game is not in progress")
	}
	if game.IsPlayersTurn() {
		return nil, fmt.Errorf("not bot's turn")
	}
	if err := game.game.MoveStr(strings.TrimSpace(move)); err != nil {
		return nil, fmt.Errorf("bot returned illegal move %q: %w", move, err)
	}
	if game.game.Outcome() != chess.NoOutcome {
		game.status = GameStatusFinished
	}
	return game, nil
}

// Resign ends the bot game with the player as the loser.
func (bm *BotGameManager) Resign(fingerprint string) (*BotGame, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	game := bm.gameForLocked(fingerprint)
	if game == nil {
		return nil, fmt.Errorf("no bot game in progress")
	}
	if game.status != GameStatusInProgress {
		return nil, fmt.Errorf("game is not in progress")
	}
	game.game.Resign(game.playerColor)
	game.status = GameStatusFinished
	return game, nil
}

// BuildBotGameRecord snapshots the bot game into a persistable record.
func (bm *BotGameManager) BuildBotGameRecord(gameID string) (common.BotGame, bool) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	game := bm.games[gameID]
	if game == nil {
		return common.BotGame{}, false
	}
	return common.BotGame{
		BotGameID:         game.id,
		PlayerFingerprint: game.fingerprint,
		PlayerUsername:    game.username,
		PlayerColor:       strings.ToLower(game.playerColor.Name()),
		BotLevel:          game.botLevel,
		PGN:               game.game.String(),
		Outcome:           string(game.game.Outcome()),
		Method:            game.game.Method().String(),
	}, true
}

// RemoveBotGame drops a finished bot game from memory. Must be called only
// after the game has been persisted (or intentionally discarded).
func (bm *BotGameManager) RemoveBotGame(gameID string) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	game := bm.games[gameID]
	if game == nil {
		return
	}
	if bm.playerGames[game.fingerprint] == gameID {
		delete(bm.playerGames, game.fingerprint)
	}
	delete(bm.games, gameID)
}

func (bm *BotGameManager) gameForLocked(fingerprint string) *BotGame {
	id := bm.playerGames[fingerprint]
	if id == "" {
		return nil
	}
	return bm.games[id]
}
