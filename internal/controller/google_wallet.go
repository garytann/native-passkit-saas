package controller

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"strings"

	"native-ios-passkit-wip/internal/service"
)

type GoogleWalletController struct {
	wallet service.GoogleWalletService
}

func NewGoogleWalletController(wallet service.GoogleWalletService) GoogleWalletController {
	return GoogleWalletController{wallet: wallet}
}

func (c GoogleWalletController) Save(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/google-wallet/passes/"
	const suffix = "/save"
	if r.Method != http.MethodGet || !hasRouteShape(r.URL.Path, prefix, suffix) {
		writeError(w, http.StatusNotFound, "Google Wallet pass route not found")
		return
	}
	memberID := trimRouteValue(r.URL.Path, prefix, suffix)
	location, err := c.wallet.SaveURL(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	http.Redirect(w, r, location, http.StatusSeeOther)
}

func (c GoogleWalletController) PublicAsset(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/public/google-wallet/assets/"
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "Google Wallet asset not found")
		return
	}
	data, err := c.wallet.Asset(r.Context(), parts[0], parts[1])
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "Google Wallet asset not found")
			return
		}
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

func (c GoogleWalletController) StampImage(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/public/google-wallet/images/"
	const suffix = "/stamps.png"
	if r.Method != http.MethodGet || !hasRouteShape(r.URL.Path, prefix, suffix) {
		writeError(w, http.StatusNotFound, "Google Wallet image not found")
		return
	}
	token := trimRouteValue(r.URL.Path, prefix, suffix)
	data, err := c.wallet.StampImage(r.Context(), token)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if len(data) == 0 {
		writeError(w, http.StatusNotFound, "Google Wallet stamp image is unavailable")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
}

type GoogleWalletAdminController struct {
	wallet     service.GoogleWalletService
	tokens     service.TokenManager
	adminToken string
	pathPrefix string
}

func NewGoogleWalletAdminController(wallet service.GoogleWalletService, tokens service.TokenManager, adminToken, pathPrefix string) GoogleWalletAdminController {
	return GoogleWalletAdminController{wallet: wallet, tokens: tokens, adminToken: strings.TrimSpace(adminToken), pathPrefix: pathPrefix}
}

func (c GoogleWalletAdminController) Routes(w http.ResponseWriter, r *http.Request) {
	if !c.authorize(w, r) {
		return
	}
	merchantID, action, ok := c.route(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "Google Wallet admin route not found")
		return
	}
	switch {
	case action == "google-wallet" && r.Method == http.MethodGet:
		configuration, err := c.wallet.Configuration(r.Context(), merchantID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, configuration)
	case action == "google-wallet/assets" && r.Method == http.MethodPut:
		c.uploadAssets(w, r, merchantID)
	case action == "google-wallet/provision" && r.Method == http.MethodPost:
		configuration, err := c.wallet.Provision(r.Context(), merchantID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, configuration)
	case action == "google-wallet/activate" && r.Method == http.MethodPost:
		configuration, err := c.wallet.Activate(r.Context(), merchantID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, configuration)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (c GoogleWalletAdminController) uploadAssets(w http.ResponseWriter, r *http.Request, merchantID string) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart request")
		return
	}
	logo, err := multipartFile(r, "program_logo", 5<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	wide, _, err := optionalMultipartFile(r, "wide_logo", 5<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	configuration, err := c.wallet.StoreAssets(r.Context(), merchantID, service.GoogleWalletAssets{ProgramLogo: logo, WideLogo: wide})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, configuration)
}

func (c GoogleWalletAdminController) authorize(w http.ResponseWriter, r *http.Request) bool {
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	if c.adminToken != "" && subtle.ConstantTimeCompare([]byte(token), []byte(c.adminToken)) == 1 {
		return true
	}
	if _, err := c.tokens.VerifyAdminToken(token); err == nil {
		return true
	}
	writeError(w, http.StatusUnauthorized, "unauthorized")
	return false
}

func (c GoogleWalletAdminController) route(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix)
	if rest == path {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] != "google-wallet" {
		return "", "", false
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}
