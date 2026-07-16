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

type Server struct {
	cfg          config.Config
	memberRepo   MemberRepository
	merchantRepo MerchantRepository
}

func NewServer(cfg config.Config, memberRepo MemberRepository, merchantRepo MerchantRepository) Server {
	return Server{
		cfg:          cfg,
		memberRepo:   memberRepo,
		merchantRepo: merchantRepo,
	}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	v1 := newRouteGroup(mux, APIV1Prefix)

	linker := newPassLinker(s.cfg.PublicBaseURL)
	memberService := service.NewMemberService(s.memberRepo, linker)
	merchantService := service.NewMerchantService(s.merchantRepo)
	passService := service.NewPassService(s.cfg, s.memberRepo, linker)

	healthController := controller.HealthController{}
	customerController := controller.NewCustomerController(memberService)
	merchantController := controller.NewMerchantController(merchantService)
	passController := controller.NewPassController(passService, v1.Path("/apple/passes/"))

	v1.HandleFunc("/healthz", method(http.MethodGet, healthController.Check))
	v1.HandleFunc("/customers/members/register", method(http.MethodPost, customerController.RegisterMember))
	v1.HandleFunc("/merchants/register", method(http.MethodPost, merchantController.RegisterMerchant))
	v1.HandleFunc("/apple/passes/example", method(http.MethodGet, passController.Example))
	v1.HandleFunc("/apple/passes/", method(http.MethodGet, passController.Routes))

	return logRequests(withCORS(mux))
}
