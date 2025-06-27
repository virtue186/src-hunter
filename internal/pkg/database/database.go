package database

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"src-hunter/internal/pkg/config"
	"src-hunter/internal/pkg/logger"
)

var db *gorm.DB

func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	log := logger.GetLogger()
	var err error
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=Asia/Shanghai",
		cfg.Host,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.Port,
		cfg.SSLMode,
	)
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	log.Infof("Database connection initialized")

	err = db.AutoMigrate()
	if err != nil {
		return nil, fmt.Errorf("failed to migrate projects: %w", err)
	}
	log.Infof("Database migration complete")
	return db, nil
}

func GetDB() *gorm.DB {
	if db == nil {
		panic("Database is not initialized")
	}
	return db
}
