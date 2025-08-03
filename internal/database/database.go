package database

import (
	"fmt"
	"github.com/src-hunter/internal/model"
	"github.com/src-hunter/pkg/config"
	"github.com/src-hunter/pkg/logger"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

func InitDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
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
	logger.Logger.Info("连接数据库成功")

	err = db.AutoMigrate(
		&model.Project{},
		&model.ProjectTarget{},
		&model.Asset{},
		&model.Domain{},
		&model.AssetDomainMapping{},
		&model.IPMetadata{},
		&model.Task{},
		&model.ScanProfile{},
		&model.TaskOutput{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate projects: %w", err)
	}
	logger.Logger.Info("数据库迁移成功")
	return db, nil
}

func GetDB() *gorm.DB {
	if db == nil {
		panic("数据库未初始化")
	}
	return db
}
