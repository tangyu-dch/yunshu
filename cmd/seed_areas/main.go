package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"yunshu/internal/infra/installer"
	"yunshu/internal/infra/system"
)

func main() {
	dsn := "root:db123456@tcp(127.0.0.1:3306)/yunshu?charset=utf8mb4&parseTime=True&loc=Local"
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
