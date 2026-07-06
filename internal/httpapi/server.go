package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/applepass"
	"native-ios-passkit-wip/internal/config"
)

var errRouteNotFound = errors.New("pass route not found")

type Server struct {
	cfg       config.Config
	generator applepass.Generator
}

func NewServer(cfg config.Config) Server {
	return Server{
		cfg: cfg,
		generator: applepass.Generator{
			SignerCertPath: cfg.PassCertPath,
			SignerKeyPath:  cfg.PassKeyPath,
			WWDRCertPath:   cfg.WWDRCertPath,
			OpenSSLBin:     cfg.OpenSSLBin,
		},
	}
}

func (s Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", method("GET", s.handleHealth))
	mux.HandleFunc("/apple/passes/example", method("GET", s.handleExampleLink))
	mux.HandleFunc("/apple/passes/", method("GET", s.handleApplePassRoutes))
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
		"passURL":    s.cfg.PublicBaseURL + "/apple/passes/member-1001.pkpass",
		"previewURL": s.cfg.PublicBaseURL + "/apple/passes/member-1001/preview",
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

	pass := s.stampCardPass(sampleStampCardMember(memberID))

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
	const prefix = "/apple/passes/"
	const suffix = ".pkpass"
	if !hasRouteShape(path, prefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, prefix, suffix), true
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
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
