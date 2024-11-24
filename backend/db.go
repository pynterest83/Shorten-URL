package main

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var DB *gorm.DB

func initDB() {
	dsn := "host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"
	var database *gorm.DB
	var err error
	database, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to PostgreSQL: %v", err))
	}
	err = database.AutoMigrate(&URL{})
	if err != nil {
		panic(fmt.Sprintf("Failed to auto-migrate database: %v", err))
	}
	DB = database

	// Configure connection pool
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
}

func batchInsert(urls []URL) {
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		for i := range urls {
			urls[i].ID = makeID()
		}

		tx := DB.Begin()
		if err := tx.Create(&urls).Error; err != nil {
			tx.Rollback()
			fmt.Printf("Batch insert failed on attempt %d: %v\n", attempt, err)
			continue
		}

		if err := tx.Commit().Error; err != nil {
			fmt.Printf("Failed to commit transaction: %v\n", err)
		} else {
			for _, url := range urls {
				if url.ResultChan != nil {
					url.ResultChan <- url.ID
				}
			}
			return
		}
	}

	for _, url := range urls {
		if url.ResultChan != nil {
			url.ResultChan <- "error"
		}
	}
}

func makeID() string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	id := make([]byte, 6)
	for i := range id {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		id[i] = charset[num.Int64()]
	}
	return string(id)
}
