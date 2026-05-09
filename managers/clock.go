package managers

import (
	"time"

	"github.com/charmbracelet/log"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/notnil/chess"
)

type ClockUpdateMsg struct {
	WhiteTime time.Duration
	BlackTime time.Duration
	GameID    string
}

type TimeForfeitMsg struct {
	GameID     string
	LoserColor chess.Color
}

type ClockManager struct {
	gameManager    *GameManager
	dataManager    *DataManager
	sessionManager SessionMessenger
	stopCh         chan struct{}
}

type SessionMessenger interface {
	GetProgram(fingerprint string) *tea.Program
}

func NewClockManager(gm *GameManager, dm *DataManager, sm SessionMessenger) *ClockManager {
	return &ClockManager{
		gameManager:    gm,
		dataManager:    dm,
		sessionManager: sm,
		stopCh:         make(chan struct{}),
	}
}

func (cm *ClockManager) Start() {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cm.stopCh:
			return
		case <-ticker.C:
			cm.tick()
		}
	}
}

func (cm *ClockManager) Stop() {
	close(cm.stopCh)
}

func (cm *ClockManager) tick() {
	games := cm.gameManager.AllInProgressGames()
	for _, game := range games {
		expired, loserColor := game.IsTimeExpired()
		if expired {
			cm.handleTimeForfeit(game, loserColor)
			continue
		}

		whiteTime, blackTime := game.CurrentClocks()
		msg := ClockUpdateMsg{
			WhiteTime: whiteTime,
			BlackTime: blackTime,
			GameID:    game.id,
		}

		if game.whitePlayer != nil {
			if prog := cm.sessionManager.GetProgram(game.whitePlayer.fingerprint); prog != nil {
				prog.Send(msg)
			}
		}
		if game.blackPlayer != nil {
			if prog := cm.sessionManager.GetProgram(game.blackPlayer.fingerprint); prog != nil {
				prog.Send(msg)
			}
		}
	}
}

func (cm *ClockManager) handleTimeForfeit(game *Game, loserColor chess.Color) {
	finished := cm.gameManager.EndByTimeForfeit(game.id, loserColor)
	if finished == nil {
		return
	}

	record := cm.gameManager.BuildGameRecord(game.id)
	record.Method = MethodTimeForfeit

	whiteFP := game.whitePlayer.fingerprint
	blackFP := game.blackPlayer.fingerprint

	cm.dataManager.AddGame(record)
	cm.gameManager.RemoveGame(game.id)

	log.Info("Time forfeit", "game", game.id, "loser_color", loserColor)

	forfeitMsg := TimeForfeitMsg{
		GameID:     game.id,
		LoserColor: loserColor,
	}

	for _, fp := range []string{whiteFP, blackFP} {
		if prog := cm.sessionManager.GetProgram(fp); prog != nil {
			prog.Send(forfeitMsg)
		}
	}
}