package main

import (
	"context"
	"log"
	"net/http"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/database"
	"native-ios-passkit-wip/internal/httpapi"
	"native-ios-passkit-wip/internal/repository"
)

func main() {
	cfg := config.Load()

	var memberRepo httpapi.MemberRepository
	if cfg.DatabaseURL == "" {
		log.Printf("DATABASE_URL is not set; member registration endpoint will be unavailable")
	} else {
		db, err := database.OpenPostgres(context.Background(), cfg.DatabaseURL)
		if err != nil {
			log.Fatalf("connect postgres: %v", err)
		}
		sqlDB, err := db.DB()
		if err != nil {
			log.Fatalf("get postgres pool: %v", err)
		}
		defer sqlDB.Close()
		if err := repository.AutoMigrateStoreMembers(db); err != nil {
			log.Fatalf("migrate store members: %v", err)
		}
		memberRepo = repository.NewStoreMemberRepository(db)
	}

	server := httpapi.NewServer(cfg, memberRepo)

	addr := ":" + cfg.Port
	log.Printf("native iOS PassKit backend listening on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, server.Routes()))
}
