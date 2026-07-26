package app

import (
	"context"
	"log"
	"net/http"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/database"
	"native-ios-passkit-wip/internal/httpapi"
	"native-ios-passkit-wip/internal/repository"
)

func NewHTTPHandler(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	var memberRepo httpapi.MemberRepository
	var merchantRepo httpapi.MerchantRepository
	var campaignRepo httpapi.CampaignRepository
	var walletRegistrationRepo httpapi.WalletRegistrationRepository
	var walletPassRepo httpapi.WalletPassRepository
	cleanup := func() {}

	if cfg.DatabaseURL == "" {
		log.Printf("DATABASE_URL is not set; database-backed endpoints will be unavailable")
	} else {
		db, err := database.OpenPostgres(ctx, cfg.DatabaseURL)
		if err != nil {
			return nil, cleanup, err
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, cleanup, err
		}

		cleanup = func() {
			_ = sqlDB.Close()
		}
		memberRepo = repository.NewStoreMemberRepository(db)
		merchantRepo = repository.NewMerchantRepository(db)
		campaignRepo = repository.NewCampaignRepository(db)
		walletRegistrationRepo = repository.NewWalletRegistrationRepository(db)
		walletPassRepo = repository.NewWalletPassRepository(db)
	}

	server := httpapi.NewServerWithRepositories(cfg, memberRepo, merchantRepo, campaignRepo, walletRegistrationRepo, walletPassRepo)
	return server.Routes(), cleanup, nil
}
