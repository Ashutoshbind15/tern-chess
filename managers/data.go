package managers

import (
	"github.com/Ashutoshbind15/ssh-chess/common"
	"gorm.io/driver/sqlite"
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
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	if err := db.AutoMigrate(&common.Player{}); err != nil {
		panic(err)
	}

	dm.db = db
}

func (dm *DataManager) GetPlayer(fingerprint string) *common.Player {
	var player common.Player
	result := dm.db.First(&player, "fingerprint = ?", fingerprint)

	if result.Error != nil {
		return nil
	}
	return &player
}

func (dm *DataManager) AddPlayer(player common.Player) {
	dm.db.Create(&player)
}

func (dm *DataManager) DeletePlayer(fingerprint string) {
	dm.db.Delete(&common.Player{}, "fingerprint = ?", fingerprint)
}
