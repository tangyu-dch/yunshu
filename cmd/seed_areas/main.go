package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunshu/internal/infra/config"
	"yunshu/internal/infra/installer"
	"yunshu/internal/infra/system"
)

func main() {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		dsn = os.Getenv("DB_DSN")
	}
	if dsn == "" {
		// 优先从 configs/default.yaml 加载
		if cfg, err := config.Load("configs/default.yaml"); err == nil && cfg.MySQL.DSN != "" {
			dsn = cfg.MySQL.DSN
		}
	}
	if dsn == "" {
		dsn = "root:@tcp(127.0.0.1:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local"
		slog.Warn("使用默认无密码 MySQL DSN，仅适用于开发环境。生产环境请设置 MYSQL_DSN/DB_DSN 环境变量或配置 configs/default.yaml")
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	err = db.AutoMigrate(&system.AreaCodeModel{})
	if err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}

	var count int64
	db.Model(&system.AreaCodeModel{}).Count(&count)
	fmt.Printf("Current count in cc_sys_area: %d\n", count)

	if count == 0 {
		fmt.Println("Seeding cc_sys_area with area seeds...")
		seeds := installer.GetAreaCodeSeeds()
		repo := system.NewAreaCodeGormRepository(db)
		err = repo.SaveBatch(context.Background(), seeds)
		if err != nil {
			log.Fatalf("Seeding failed: %v", err)
		}
		fmt.Printf("Successfully seeded %d area codes.\n", len(seeds))
	} else {
		fmt.Println("cc_sys_area is already seeded.")
	}
}
