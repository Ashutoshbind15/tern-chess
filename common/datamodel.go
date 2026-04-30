package common

import (
	"gorm.io/gorm"
)

type Player struct {
	gorm.Model
	Fingerprint string `gorm:"uniqueIndex"`
	Username    string
}

