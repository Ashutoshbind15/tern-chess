package managers

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Ashutoshbind15/ssh-chess/common"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DataManager struct {
	db *gorm.DB
}

func NewDataManager() *DataManager {
	dm := &DataManager{}
	dm.Init()
	return dm
}

func (dm *DataManager) Init() {
	dbURL := strings.TrimSpace(os.Getenv("DB_URL"))
	if dbURL == "" {
		panic("DB_URL environment variable is required")
	}

	db, err := gorm.Open(postgres.Open(dbURL), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("failed to connect database: %v", err))
	}

	if err := db.AutoMigrate(&common.Player{}, &common.Game{}, &common.BotGame{}); err != nil {
		panic(err)
	}

	dm.db = db
}

func (dm *DataManager) GetPlayer(fingerprint string) (*common.Player, error) {
	var player common.Player
	result := dm.db.First(&player, "fingerprint = ?", fingerprint)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &player, nil
}

func (dm *DataManager) AddPlayer(player common.Player) error {
	return dm.db.Create(&player).Error
}

func (dm *DataManager) DeletePlayer(fingerprint string) {
	dm.db.Delete(&common.Player{}, "fingerprint = ?", fingerprint)
}

func (dm *DataManager) AddGame(game common.Game) error {
	return dm.db.Create(&game).Error
}

func (dm *DataManager) GetGamesForPlayer(fingerprint string) ([]common.Game, error) {
	var games []common.Game
	result := dm.db.
		Where("white_fingerprint = ? OR black_fingerprint = ?", fingerprint, fingerprint).
		Order("created_at DESC").
		Find(&games)
	if result.Error != nil {
		return nil, result.Error
	}
	return games, nil
}

func (dm *DataManager) AddBotGame(game common.BotGame) error {
	return dm.db.Create(&game).Error
}

func (dm *DataManager) GetBotGamesForPlayer(fingerprint string) ([]common.BotGame, error) {
	var games []common.BotGame
	result := dm.db.
		Where("player_fingerprint = ?", fingerprint).
		Order("created_at DESC").
		Find(&games)
	if result.Error != nil {
		return nil, result.Error
	}
	return games, nil
}
