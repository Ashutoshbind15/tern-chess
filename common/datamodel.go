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
