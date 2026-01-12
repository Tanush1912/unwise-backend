package main

import (
	"context"
	"fmt"
	"log"
	"unwise-backend/config"
	"unwise-backend/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	rows, err := db.Pool.Query(context.Background(), "SELECT id, email, name FROM users ORDER BY email ASC")
	if err != nil {
		log.Fatalf("Query failed: %v", err)
	}
	defer rows.Close()

	fmt.Println("Seeded Users in Database:")
	fmt.Println("-------------------------")
	for rows.Next() {
		var id, email, name string
		if err := rows.Scan(&id, &email, &name); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ID: %s | Email: %-20s | Name: %s\n", id, email, name)
	}
}
