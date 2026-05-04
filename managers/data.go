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

	if err := db.AutoMigrate(&common.Player{}); err != nil {
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
