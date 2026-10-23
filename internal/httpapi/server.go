package httpapi

import (
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/controller"
	"native-ios-passkit-wip/internal/service"
)

type MemberRepository interface {
	service.MemberRepository
}

type MerchantRepository interface {
	service.MerchantRepository
}

type CampaignRepository interface {
	service.CampaignRepository
	service.ActiveCampaignRepository
	service.GoogleCampaignRepository
}

type WalletRegistrationRepository interface {
	service.WalletRegistrationRepository
	service.CampaignWalletRegistrationRepository
	service.PassDesignWalletRegistrationRepository
}

type WalletPassRepository interface {
	service.WalletPassRepository
	service.PassDesignWalletPassRepository
}

type AppleWalletRepository interface {
	service.AppleWalletRepository
}

type GoogleWalletRepository interface {
	service.GoogleWalletRepository
}

type Server struct {
	cfg                    config.Config
	memberRepo             MemberRepository
	merchantRepo           MerchantRepository
	campaignRepo           CampaignRepository
	walletRegistrationRepo WalletRegistrationRepository
	walletPassRepo         WalletPassRepository
	appleWalletRepo        AppleWalletRepository
	googleWalletService    *service.GoogleWalletService
}

func (s Server) WithGoogleWalletService(wallet *service.GoogleWalletService) Server {
	s.googleWalletService = wallet
	return s
}

func NewServer(cfg config.Config, memberRepo MemberRepository, merchantRepo MerchantRepository) Server {
	return NewServerWithRepositories(cfg, memberRepo, merchantRepo, nil, nil, nil)
}

func NewServerWithRepositories(cfg config.Config, memberRepo MemberRepository, merchantRepo MerchantRepository, campaignRepo CampaignRepository, walletRegistrationRepo WalletRegistrationRepository, walletPassRepo WalletPassRepository) Server {
	return Server{
		cfg:                    cfg,
		memberRepo:             memberRepo,
		merchantRepo:           merchantRepo,
		campaignRepo:           campaignRepo,
		walletRegistrationRepo: walletRegistrationRepo,
		walletPassRepo:         walletPassRepo,
	}
}

func NewServerWithAppleWalletRepository(cfg config.Config, memberRepo MemberRepository, merchantRepo MerchantRepository, campaignRepo CampaignRepository, walletRegistrationRepo WalletRegistrationRepository, walletPassRepo WalletPassRepository, appleWalletRepo AppleWalletRepository) Server {
	server := NewServerWithRepositories(cfg, memberRepo, merchantRepo, campaignRepo, walletRegistrationRepo, walletPassRepo)
	server.appleWalletRepo = appleWalletRepo
	return server
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	v1 := newRouteGroup(mux, APIV1Prefix)

	linker := newPassLinker(s.cfg.PublicBaseURL)
	tokenManager := service.NewTokenManager(s.cfg.AuthTokenSecret)
	appleWalletService := service.NewAppleWalletService(s.cfg, s.appleWalletRepo)
	passService := service.NewPassServiceWithWalletPassesAndCampaigns(s.cfg, s.memberRepo, s.walletPassRepo, s.campaignRepo, linker, appleWalletService)
	apnsSender := service.NewAPNSPassUpdateSender(s.cfg, appleWalletService)
	merchantPassDesignService := service.NewMerchantPassDesignService(appleWalletService, s.walletPassRepo, s.walletRegistrationRepo, apnsSender)
	if s.googleWalletService != nil {
		merchantPassDesignService = merchantPassDesignService.WithProgramUpdater(*s.googleWalletService).WithSharedPrograms(*s.googleWalletService)
	}
	var passUpdater service.WalletPassUpdater = passService
	if s.googleWalletService != nil {
		passUpdater = service.NewCompositeWalletPassUpdater(passService, *s.googleWalletService)
	}
	var programProvider service.StampGoalProvider = appleWalletService
	if s.googleWalletService != nil {
		programProvider = *s.googleWalletService
	}
	memberService := service.NewMemberServiceWithWalletUpdatesAndGoals(s.memberRepo, s.merchantRepo, linker, passUpdater, s.walletRegistrationRepo, apnsSender, programProvider)
	if s.appleWalletRepo != nil {
		memberService = memberService.WithAppleRegistrationWallet(appleWalletService)
	}
	if s.googleWalletService != nil {
		memberService = memberService.WithRegistrationWallet(*s.googleWalletService)
	}
	merchantService := service.NewMerchantService(s.merchantRepo, tokenManager, s.cfg.TeamID)
	adminOnboardingService := service.NewAdminOnboardingService(merchantService, appleWalletService)
	walletService := service.NewWalletService(s.walletRegistrationRepo)
	campaignService := service.NewCampaignServiceWithPush(s.campaignRepo, s.memberRepo, passService, s.walletRegistrationRepo, apnsSender)
	if s.googleWalletService != nil {
		campaignService = campaignService.WithCampaignNotifier(*s.googleWalletService)
	}

	healthController := controller.HealthController{}
	customerController := controller.NewCustomerController(memberService, tokenManager, v1.Path("/public/merchants/"))
	merchantController := controller.NewMerchantController(merchantService)
	merchantPassDesignController := controller.NewMerchantPassDesignController(merchantPassDesignService, tokenManager)
	passController := controller.NewPassController(passService, walletService, v1.Path("/apple/passes/"))
	campaignController := controller.NewCampaignController(campaignService, tokenManager)
	adminAuthController := controller.NewAdminAuthController(s.cfg.PlatformAdminEmail, s.cfg.PlatformAdminPassword, tokenManager)
	appleWalletAdminController := controller.NewAppleWalletAdminController(appleWalletService, adminOnboardingService, tokenManager, s.cfg.AdminAPIToken, v1.Path("/admin/merchants/"))
	googleWalletService := service.GoogleWalletService{}
	if s.googleWalletService != nil {
		googleWalletService = *s.googleWalletService
	}
	googleWalletController := controller.NewGoogleWalletController(googleWalletService)
	googleWalletAdminController := controller.NewGoogleWalletAdminController(googleWalletService, tokenManager, s.cfg.AdminAPIToken, v1.Path("/admin/merchants/"))

	v1.HandleFunc("/healthz", method(http.MethodGet, healthController.Check))
	v1.HandleFunc("/customers/members", method(http.MethodGet, customerController.ListMembers))
	v1.HandleFunc("/customers/members/register", method(http.MethodPost, customerController.RegisterMember))
	v1.HandleFunc("/customers/members/", customerController.MemberRoutes)
	v1.HandleFunc("/campaigns", campaignController.Routes)
	v1.HandleFunc("/public/merchants/", method(http.MethodPost, customerController.RegisterPublicMember))
	v1.HandleFunc("/merchants/register", method(http.MethodPost, merchantController.RegisterMerchant))
	v1.HandleFunc("/merchants/login", method(http.MethodPost, merchantController.LoginMerchant))
	v1.HandleFunc("/merchants/me/apple-wallet/pass-design", merchantPassDesignController.Routes)
	v1.HandleFunc("/merchants/me/apple-wallet/stamp-icon", merchantPassDesignController.StampIcon)
	v1.HandleFunc("/merchants/me/loyalty-program", merchantPassDesignController.Routes)
	v1.HandleFunc("/merchants/me/loyalty-program/stamp-icon", merchantPassDesignController.StampIcon)
	v1.HandleFunc("/admin/auth/login", method(http.MethodPost, adminAuthController.Login))
	v1.HandleFunc("/admin/merchants", method(http.MethodGet, appleWalletAdminController.ListMerchants))
	v1.HandleFunc("/admin/merchants/", func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/google-wallet") {
			googleWalletAdminController.Routes(w, r)
			return
		}
		appleWalletAdminController.Routes(w, r)
	})
	v1.HandleFunc("/google-wallet/passes/", googleWalletController.Save)
	v1.HandleFunc("/public/google-wallet/assets/", googleWalletController.PublicAsset)
	v1.HandleFunc("/public/google-wallet/images/", googleWalletController.StampImage)
	v1.HandleFunc("/apple/passes/example", method(http.MethodGet, passController.Example))
	v1.HandleFunc("/apple/passes/", passController.Routes)

	return logRequests(withCORS(mux))
}
