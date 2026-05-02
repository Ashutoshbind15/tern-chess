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

type GameManager struct {
	games   map[string]*Game
	players map[string]*GamePlayer
}

type PlayerGameState struct {
	CurrentGameID string
	GameStatus    string
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

func (gm *GameManager) GetPlayerGameState(fingerprint string) PlayerGameState {
	player := gm.players[fingerprint]
	if player == nil || player.currentGameId == "" {
		return PlayerGameState{}
	}

	game := gm.games[player.currentGameId]
	if game == nil {
		return PlayerGameState{
			CurrentGameID: player.currentGameId,
		}
	}

	return PlayerGameState{
		CurrentGameID: player.currentGameId,
		GameStatus:    game.status,
	}
}
