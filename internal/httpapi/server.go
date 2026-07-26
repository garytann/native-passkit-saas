package httpapi

import (
	"net/http"

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
}

type WalletRegistrationRepository interface {
	service.WalletRegistrationRepository
}

type WalletPassRepository interface {
	service.WalletPassRepository
}

type Server struct {
	cfg                    config.Config
	memberRepo             MemberRepository
	merchantRepo           MerchantRepository
	campaignRepo           CampaignRepository
	walletRegistrationRepo WalletRegistrationRepository
	walletPassRepo         WalletPassRepository
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

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	v1 := newRouteGroup(mux, APIV1Prefix)

	linker := newPassLinker(s.cfg.PublicBaseURL)
	tokenManager := service.NewTokenManager(s.cfg.AuthTokenSecret)
	memberService := service.NewMemberServiceWithMerchants(s.memberRepo, s.merchantRepo, linker)
	merchantService := service.NewMerchantService(s.merchantRepo, tokenManager)
	passService := service.NewPassServiceWithWalletPassesAndCampaigns(s.cfg, s.memberRepo, s.walletPassRepo, s.campaignRepo, linker)
	walletService := service.NewWalletService(s.walletRegistrationRepo)
	campaignService := service.NewCampaignService(s.campaignRepo, s.memberRepo, passService)

	healthController := controller.HealthController{}
	customerController := controller.NewCustomerController(memberService, tokenManager, v1.Path("/public/merchants/"))
	merchantController := controller.NewMerchantController(merchantService)
	passController := controller.NewPassController(passService, walletService, v1.Path("/apple/passes/"))
	campaignController := controller.NewCampaignController(campaignService, tokenManager)

	v1.HandleFunc("/healthz", method(http.MethodGet, healthController.Check))
	v1.HandleFunc("/customers/members", method(http.MethodGet, customerController.ListMembers))
	v1.HandleFunc("/customers/members/register", method(http.MethodPost, customerController.RegisterMember))
	v1.HandleFunc("/customers/members/", method(http.MethodPut, customerController.UpdateMemberStamps))
	v1.HandleFunc("/campaigns", campaignController.Routes)
	v1.HandleFunc("/public/merchants/", method(http.MethodPost, customerController.RegisterPublicMember))
	v1.HandleFunc("/merchants/register", method(http.MethodPost, merchantController.RegisterMerchant))
	v1.HandleFunc("/merchants/login", method(http.MethodPost, merchantController.LoginMerchant))
	v1.HandleFunc("/apple/passes/example", method(http.MethodGet, passController.Example))
	v1.HandleFunc("/apple/passes/", passController.Routes)

	return logRequests(withCORS(mux))
}
