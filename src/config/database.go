package config

import (
	"fmt"
	"log"

	"collabpm/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func ConnectMySQL(cfg *Config) (*gorm.DB, error) {
	baseDSN := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort,
	)
	base, err := gorm.Open(mysql.Open(baseDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to mysql server: %w", err)
	}

	sqlDB, err := base.DB()
	if err != nil {
		return nil, err
	}
	defer sqlDB.Close()

	createDB := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		cfg.DBName,
	)
	if _, err := sqlDB.Exec(createDB); err != nil {
		return nil, fmt.Errorf("create database %s: %w", cfg.DBName, err)
	}

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database %s: %w", cfg.DBName, err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Workspace{},
		&models.WorkspaceMember{},
		&models.Project{},
		&models.ProjectStatus{},
		&models.Task{},
		&models.TaskAssignee{},
		&models.Comment{},
		&models.Notification{},
		&models.ActivityLog{},
		&models.TaskDocState{},
	); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	log.Println("database connected and migrated")
	return db, nil
}
