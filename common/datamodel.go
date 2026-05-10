package common

import (
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	Fingerprint string `gorm:"uniqueIndex"`
	Username    string
}

type Game struct {
	gorm.Model
	GameID           string `gorm:"uniqueIndex"`
	WhiteFingerprint string `gorm:"index"`
	WhiteUsername    string
	BlackFingerprint string `gorm:"index"`
	BlackUsername    string
	PGN              string
	Outcome          string
	Method           string
	TimeControl      int
}

type BotGame struct {
	gorm.Model
	BotGameID         string `gorm:"uniqueIndex"`
	PlayerFingerprint string `gorm:"index"`
	PlayerUsername    string
	PlayerColor       string // "white" or "black"
	BotLevel          int
	PGN               string
	Outcome           string
	Method            string
}
