package controller

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/service"
)

type AppleWalletAdminController struct {
	wallet     service.AppleWalletService
	adminToken string
	pathPrefix string
}

func NewAppleWalletAdminController(wallet service.AppleWalletService, adminToken string, pathPrefix string) AppleWalletAdminController {
	return AppleWalletAdminController{
		wallet:     wallet,
		adminToken: strings.TrimSpace(adminToken),
		pathPrefix: pathPrefix,
	}
}

type configureAppleWalletIssuerRequest struct {
	OwnershipMode    string `json:"ownership_mode"`
	TeamIdentifier   string `json:"team_identifier"`
	OrganizationName string `json:"organization_name"`
}

type createApplePassTypeRequest struct {
	PassTypeIdentifier string `json:"pass_type_identifier"`
	PassStyle          string `json:"pass_style"`
}

type provisionAppleCertificateRequest struct {
	PassTypeIdentifier string `json:"pass_type_identifier"`
	CredentialAlias    string `json:"credential_alias"`
}

func (c AppleWalletAdminController) Routes(w http.ResponseWriter, r *http.Request) {
	if !c.authorize(w, r) {
		return
	}
	merchantID, action, ok := c.route(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, "apple wallet admin route not found")
		return
	}

	switch {
	case action == "" && r.Method == http.MethodGet:
		c.configuration(w, r, merchantID)
	case action == "issuer" && r.Method == http.MethodPost:
		c.configureIssuer(w, r, merchantID)
	case action == "pass-types" && r.Method == http.MethodPost:
		c.createPassType(w, r, merchantID)
	case action == "certificates" && r.Method == http.MethodPost:
		c.provisionCertificate(w, r, merchantID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (c AppleWalletAdminController) configuration(w http.ResponseWriter, r *http.Request, merchantID string) {
	configuration, err := c.wallet.Configuration(r.Context(), merchantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, configuration)
}

func (c AppleWalletAdminController) configureIssuer(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request configureAppleWalletIssuerRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	issuer, err := c.wallet.ConfigureIssuer(r.Context(), service.ConfigureAppleWalletIssuerInput{
		MerchantID:       merchantID,
		OwnershipMode:    request.OwnershipMode,
		TeamIdentifier:   request.TeamIdentifier,
		OrganizationName: request.OrganizationName,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, issuer)
}

func (c AppleWalletAdminController) createPassType(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request createApplePassTypeRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	passType, err := c.wallet.CreatePassType(r.Context(), service.CreateApplePassTypeRequest{
		MerchantID:         merchantID,
		PassTypeIdentifier: request.PassTypeIdentifier,
		PassStyle:          request.PassStyle,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, passType)
}

func (c AppleWalletAdminController) provisionCertificate(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request provisionAppleCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	certificate, err := c.wallet.ProvisionCertificate(r.Context(), service.ProvisionAppleCertificateRequest{
		MerchantID:         merchantID,
		PassTypeIdentifier: request.PassTypeIdentifier,
		CredentialAlias:    request.CredentialAlias,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, certificate)
}

func (c AppleWalletAdminController) authorize(w http.ResponseWriter, r *http.Request) bool {
	if c.adminToken == "" {
		writeError(w, http.StatusServiceUnavailable, "admin API token is not configured")
		return false
	}
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil || subtle.ConstantTimeCompare([]byte(token), []byte(c.adminToken)) != 1 {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func (c AppleWalletAdminController) route(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix)
	if rest == path {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" || parts[1] != "apple-wallet" {
		return "", "", false
	}
	action := ""
	if len(parts) == 3 {
		action = parts[2]
	}
	return parts[0], action, true
}
