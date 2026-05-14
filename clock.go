package main

import (
	"time"

	"github.com/charmbracelet/log"
	"github.com/notnil/chess"

	"github.com/Ashutoshbind15/ssh-chess/managers"
)

// ClockUpdateMsg / TimeForfeitMsg are page messages emitted by the clock
// ticker. They live in the cmd package next to the rest of the page msg
// types because the ticker that produces them is itself a cmd-level
// composition of GameManager + DataManager + SessionManager.
type ClockUpdateMsg struct {
	WhiteTime time.Duration
	BlackTime time.Duration
	GameID    string
}

type TimeForfeitMsg struct {
	GameID     string
	LoserColor chess.Color
	Snapshot   *managers.Snapshot
}

// runClockTicker drives in-progress game clocks. It is intentionally cmd
// code rather than a manager: it stitches GameManager (state), DataManager
// (persistence) and SessionManager (delivery) together, but none of those
// know about each other. Stops when stopCh is closed.
func runClockTicker(stopCh <-chan struct{}) {
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			tickClocks()
		}
	}
}

// tickClocks snapshots every in-progress game once (under the manager
// lock) and then walks the snapshots without further coordination. The
// previous version called CurrentClocks/IsTimeExpired on live *Game
// pointers without holding gm.mu, which races against MakeMove.
func tickClocks() {
	for _, info := range gameManager.TickAll() {
		if info.Expired {
			handleTimeForfeit(info)
			continue
		}

		msg := ClockUpdateMsg{
			WhiteTime: info.WhiteTime,
			BlackTime: info.BlackTime,
			GameID:    info.GameID,
		}
		for _, fp := range []string{info.WhiteFingerprint, info.BlackFingerprint} {
			if fp == "" {
				continue
			}
			if prog := sessionManager.GetProgram(fp); prog != nil {
				prog.Send(msg)
			}
		}
	}
}

func handleTimeForfeit(info managers.TickInfo) {
	gameID := info.GameID
	finished := gameManager.EndByTimeForfeit(gameID, info.LoserColor)
	if finished == nil {
		return
	}

	record := gameManager.BuildGameRecord(gameID)
	record.Method = managers.MethodTimeForfeit
	if err := dataManager.AddGame(record); err != nil {
		log.Error("failed to persist time-forfeit game", "id", gameID, "error", err)
	}
	gameManager.RemoveGame(gameID)

	log.Info("Time forfeit", "game", gameID, "loser_color", info.LoserColor)

	forfeitMsg := TimeForfeitMsg{
		GameID:     gameID,
		LoserColor: info.LoserColor,
		Snapshot:   finished,
	}
	for _, fp := range []string{info.WhiteFingerprint, info.BlackFingerprint} {
		if fp == "" {
			continue
		}
		if prog := sessionManager.GetProgram(fp); prog != nil {
			prog.Send(forfeitMsg)
		}
	}
}
