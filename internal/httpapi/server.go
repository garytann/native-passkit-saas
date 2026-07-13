package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/httpapi/customer"
)

var errRouteNotFound = errors.New("pass route not found")

const apiVersionPrefix = "/api/v1"
const applePassesPathPrefix = apiVersionPrefix + "/apple/passes/"

type MemberRepository interface {
	customer.Repository
	GetByID(ctx context.Context, memberID string) (domain.StoreMember, error)
}

type Server struct {
	cfg        config.Config
	generator  applepass.Generator
	memberRepo MemberRepository
}

func NewServer(cfg config.Config, memberRepo MemberRepository) Server {
	return Server{
		cfg: cfg,
		generator: applepass.Generator{
			SignerCertPath: cfg.PassCertPath,
			SignerKeyPath:  cfg.PassKeyPath,
			WWDRCertPath:   cfg.WWDRCertPath,
			OpenSSLBin:     cfg.OpenSSLBin,
		},
		memberRepo: memberRepo,
	}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	customerHandler := customer.NewHandler(s.memberRepo, s.publicAPIBaseURL())

	mux.HandleFunc(apiVersionPrefix+"/healthz", method("GET", s.handleHealth))
	mux.HandleFunc(apiVersionPrefix+"/customers/members/register", method("POST", customerHandler.RegisterMember))
	mux.HandleFunc(apiVersionPrefix+"/apple/passes/example", method("GET", s.handleExampleLink))
	mux.HandleFunc(applePassesPathPrefix, method("GET", s.handleApplePassRoutes))
	return logRequests(withCORS(mux))
}

func (s Server) handleApplePassRoutes(w http.ResponseWriter, r *http.Request) {
	if _, ok := memberIDFromPreviewPath(r.URL.Path); ok {
		s.handleApplePassPreview(w, r)
		return
	}
	if _, ok := memberIDFromQRCodePath(r.URL.Path); ok {
		s.handleApplePassQRCode(w, r)
		return
	}
	s.handleApplePass(w, r)
}

func (s Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (s Server) handleExampleLink(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"passURL":    s.passURL("member-1001"),
		"previewURL": s.previewURL("member-1001"),
	})
}

func (s Server) handleApplePass(w http.ResponseWriter, r *http.Request) {
	memberID, ok := memberIDFromPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound)
		return
	}
	if memberID == "" {
		writeError(w, http.StatusBadRequest, errors.New("member id is required"))
		return
	}

	member, err := s.stampCardMemberForID(r.Context(), memberID)
	if err != nil {
		writeMemberLookupError(w, err)
		return
	}

	pass := s.stampCardPass(member)

	data, err := s.generator.Create(pass)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.pkpass")
	w.Header().Set("Content-Disposition", `attachment; filename="`+memberID+`.pkpass"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func memberIDFromPath(path string) (string, bool) {
	const suffix = ".pkpass"
	if !hasRouteShape(path, applePassesPathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, applePassesPathPrefix, suffix), true
}

func hasRouteShape(path string, prefix string, suffix string) bool {
	return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
}

func trimRouteValue(path string, prefix string, suffix string) string {
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.TrimSpace(value)
}

func sampleStampCardMember(memberID string) applepass.StampCardMember {
	return applepass.StampCardMember{
		ID:         memberID,
		FullName:   "Taylor Example",
		BrandName:  "Daily Brew",
		RewardName: "Free Coffee",
		StampCount: 6,
		StampGoal:  10,
		BarcodeID:  "stamp-card:" + memberID,
	}
}

func (s Server) stampCardMemberForID(ctx context.Context, memberID string) (applepass.StampCardMember, error) {
	if s.memberRepo == nil {
		return sampleStampCardMember(memberID), nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	member, err := s.memberRepo.GetByID(ctx, memberID)
	if err != nil {
		return applepass.StampCardMember{}, err
	}

	return storeMemberToStampCard(member), nil
}

func storeMemberToStampCard(member domain.StoreMember) applepass.StampCardMember {
	return applepass.StampCardMember{
		ID:         member.MemberID,
		FullName:   member.CustomerName,
		BrandName:  member.Merchant,
		RewardName: "Free Coffee",
		StampCount: member.StampProgress,
		StampGoal:  10,
		BarcodeID:  "stamp-card:" + member.MemberID,
	}
}

func (s Server) passURL(memberID string) string {
	return s.publicAPIBaseURL() + "/apple/passes/" + memberID + ".pkpass"
}

func (s Server) previewURL(memberID string) string {
	return s.publicAPIBaseURL() + "/apple/passes/" + memberID + "/preview"
}

func (s Server) qrCodeURL(memberID string) string {
	return s.publicAPIBaseURL() + "/apple/passes/" + memberID + "/qr.png"
}

func (s Server) publicAPIBaseURL() string {
	return strings.TrimRight(s.cfg.PublicBaseURL, "/") + apiVersionPrefix
}

func writeMemberLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, domain.ErrStoreMemberNotFound) {
		writeError(w, http.StatusNotFound, errors.New("member not found"))
		return
	}
	writeError(w, http.StatusInternalServerError, err)
}

func (s Server) stampCardPass(member applepass.StampCardMember) applepass.Pass {
	return applepass.NewStampCardPass(applepass.TemplateConfig{
		PassTypeID:       s.cfg.PassTypeID,
		TeamID:           s.cfg.TeamID,
		OrganizationName: s.cfg.OrganizationName,
		WebServiceURL:    s.cfg.WebServiceURL,
	}, member)
}

func method(allowed string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != allowed {
			w.Header().Set("Allow", allowed)
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		handler(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
