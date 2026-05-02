package managers

import (
	"fmt"
	"math/rand"

	"github.com/google/uuid"
	"github.com/notnil/chess"
)

const (
	GameStatusWaiting    = "waiting"
	GameStatusInProgress = "in progress"
	GameStatusFinished   = "finished"
)

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

type GameManager struct {
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

func (gm *GameManager) CreateGame(fingerprint string) (string, error) {
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
			id:          gameId,
			whitePlayer: player,
			blackPlayer: nil,
			status:      GameStatusWaiting,
			game:        chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		}
	} else {
		gm.games[gameId] = &Game{
			id:          gameId,
			whitePlayer: nil,
			blackPlayer: player,
			status:      GameStatusWaiting,
			game:        chess.NewGame(chess.UseNotation(chess.UCINotation{})),
		}
	}

	return gameId, nil
}

func (gm *GameManager) GetHalfGame() *Game {
	for _, game := range gm.games {
		if game.status == GameStatusWaiting {
			return game
		}
	}
	return nil
}

func (gm *GameManager) JoinRandomGame(fingerprint string) (string, error) {
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
	player.currentGameId = halfGame.id

	return halfGame.id, nil
}

func (gm *GameManager) JoinGame(fingerprint string, gameId string) (string, error) {
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
	player.currentGameId = gameId

	return gameId, nil
}

func (gm *GameManager) MakeMove(fingerprint string, move string) (*Game, error) {
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
	if err := game.game.MoveStr(move); err != nil {
		return nil, err
	}
	if game.game.Outcome() != chess.NoOutcome {
		game.status = GameStatusFinished
	} else {
		game.status = GameStatusInProgress
	}
	return game, nil
}

func (gm *GameManager) GameForPlayer(fingerprint string) *Game {
	player := gm.players[fingerprint]
	if player == nil || player.currentGameId == "" {
		return nil
	}
	return gm.games[player.currentGameId]
}

func (gm *GameManager) OpponentFingerprint(gameID string, fingerprint string) string {
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
