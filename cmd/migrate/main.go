package main

import (
	"context"
	"log"
	"path/filepath"
	"time"

	"codeatlas/packages/database"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dbCfg, err := database.LoadPostgresConfigFromEnv()
	if err != nil {
		log.Fatalf("load postgres config: %v", err)
	}

	db, err := database.NewPostgresPool(ctx, dbCfg)
	if err != nil {
		log.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	migrationsDir := filepath.Join("infra", "postgres", "migrations")
	migrations, err := database.LoadMigrations(migrationsDir)
	if err != nil {
		log.Fatalf("load migrations: %v", err)
	}

	if len(migrations) == 0 {
		log.Printf("no migrations found in %s", migrationsDir)
		return
	}

	if err := database.ApplyMigrations(ctx, db, migrations); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	log.Printf("applied migrations successfully from %s", migrationsDir)
}
