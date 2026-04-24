package database

import (
	"log"

	"lazymanga/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDB(dbPath string) *gorm.DB {
	var err error
	log.Printf("InitDB: opening sqlite database at %s", dbPath)
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// 调试用：每次启动都删除并重建 ISOs 表
	// !!! 生产环境请移除此段 !!!
	/*
		if err := DB.Migrator().DropTable(&models.ISOs{}); err != nil {
			log.Printf("DropTable ISOs failed: %v", err)
		}*/

	log.Printf("InitDB: running AutoMigrate for tables=isos,repositories,repo_type_defs")
	err = DB.AutoMigrate(&models.ISOs{}, &models.Repository{}, &models.RepoTypeDef{})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("InitDB: AutoMigrate finished for tables=isos,repositories,repo_type_defs")

	return DB
}
