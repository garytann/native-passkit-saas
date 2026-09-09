package controller

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/service"
)

type AppleWalletAdminController struct {
	wallet     service.AppleWalletService
	onboarding service.AdminOnboardingService
	tokens     service.TokenManager
	adminToken string
	pathPrefix string
}

func NewAppleWalletAdminController(wallet service.AppleWalletService, onboarding service.AdminOnboardingService, tokens service.TokenManager, adminToken string, pathPrefix string) AppleWalletAdminController {
	return AppleWalletAdminController{
		wallet:     wallet,
		onboarding: onboarding,
		tokens:     tokens,
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

type upsertApplePassDesignRequest struct {
	PassTypeIdentifier  string `json:"pass_type_identifier"`
	ProgramName         string `json:"program_name"`
	PassDescription     string `json:"pass_description"`
	StampGoal           int    `json:"stamp_goal"`
	BackgroundColor     string `json:"background_color"`
	ForegroundColor     string `json:"foreground_color"`
	LabelColor          string `json:"label_color"`
	AssetBundleRef      string `json:"asset_bundle_ref"`
	SupportText         string `json:"support_text"`
	TermsText           string `json:"terms_text"`
	CampaignDisplayMode string `json:"campaign_display_mode"`
}

type updatePaymentStatusRequest struct {
	Status string `json:"status"`
}

type activateAppleWalletRequest struct {
	PassTypeIdentifier string `json:"pass_type_identifier"`
}

func (c AppleWalletAdminController) ListMerchants(w http.ResponseWriter, r *http.Request) {
	if !c.authorize(w, r) {
		return
	}
	merchants, err := c.onboarding.ListMerchants(r.Context())
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"merchants": merchants})
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
	case action == "onboarding" && r.Method == http.MethodGet:
		c.onboardingStatus(w, r, merchantID)
	case action == "payment-status" && r.Method == http.MethodPut:
		c.updatePaymentStatus(w, r, merchantID)
	case action == "apple-wallet" && r.Method == http.MethodGet:
		c.configuration(w, r, merchantID)
	case action == "apple-wallet/issuer" && r.Method == http.MethodPost:
		c.configureIssuer(w, r, merchantID)
	case action == "apple-wallet/pass-types" && r.Method == http.MethodPost:
		c.createPassType(w, r, merchantID)
	case action == "apple-wallet/certificates" && r.Method == http.MethodPost:
		c.provisionCertificate(w, r, merchantID)
	case action == "apple-wallet/certificate-files" && r.Method == http.MethodPost:
		c.uploadCertificateFiles(w, r, merchantID)
	case action == "apple-wallet/pass-assets" && r.Method == http.MethodPut:
		c.uploadPassAssets(w, r, merchantID)
	case action == "apple-wallet/activate" && r.Method == http.MethodPost:
		c.activate(w, r, merchantID)
	case action == "apple-wallet/pass-design" && r.Method == http.MethodGet:
		c.passDesign(w, r, merchantID)
	case action == "apple-wallet/pass-design" && r.Method == http.MethodPut:
		c.upsertPassDesign(w, r, merchantID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (c AppleWalletAdminController) onboardingStatus(w http.ResponseWriter, r *http.Request, merchantID string) {
	status, err := c.onboarding.Status(r.Context(), merchantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (c AppleWalletAdminController) updatePaymentStatus(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request updatePaymentStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	status, err := c.onboarding.UpdatePaymentStatus(r.Context(), merchantID, request.Status)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
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

func (c AppleWalletAdminController) uploadCertificateFiles(w http.ResponseWriter, r *http.Request, merchantID string) {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "certificate upload must be multipart form data")
		return
	}
	status, err := c.onboarding.Status(r.Context(), merchantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	certificate, err := multipartFile(r, "pass_certificate", 2<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	privateKey, err := multipartFile(r, "private_key", 2<<20)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	alias, err := service.NewAppleStorageAlias(status.Merchant.MerchantSlug)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if err := c.wallet.StoreCertificateFiles(alias, certificate, privateKey); err != nil {
		writeServiceError(w, err)
		return
	}
	validated, err := c.wallet.ProvisionCertificate(r.Context(), service.ProvisionAppleCertificateRequest{
		MerchantID:         merchantID,
		PassTypeIdentifier: r.FormValue("pass_type_identifier"),
		CredentialAlias:    alias,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"credential_alias": alias, "certificate": validated})
}

func (c AppleWalletAdminController) uploadPassAssets(w http.ResponseWriter, r *http.Request, merchantID string) {
	if err := r.ParseMultipartForm(24 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "asset upload must be multipart form data")
		return
	}
	status, err := c.onboarding.Status(r.Context(), merchantID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	assets := make(map[string][]byte, 6)
	for _, name := range []string{"icon.png", "icon@2x.png", "icon@3x.png", "logo.png", "logo@2x.png", "logo@3x.png"} {
		data, err := multipartFile(r, name, 4<<20)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		assets[name] = data
	}
	alias, err := service.NewAppleStorageAlias(status.Merchant.MerchantSlug)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if err := c.wallet.StorePassAssets(alias, assets); err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"asset_bundle_ref": alias})
}

func (c AppleWalletAdminController) activate(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request activateAppleWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	status, err := c.onboarding.Activate(r.Context(), merchantID, request.PassTypeIdentifier)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (c AppleWalletAdminController) passDesign(w http.ResponseWriter, r *http.Request, merchantID string) {
	design, err := c.wallet.PassDesign(r.Context(), merchantID, r.URL.Query().Get("pass_type_identifier"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, design)
}

func (c AppleWalletAdminController) upsertPassDesign(w http.ResponseWriter, r *http.Request, merchantID string) {
	var request upsertApplePassDesignRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	design, err := c.wallet.UpsertPassDesign(r.Context(), service.UpsertApplePassDesignRequest{
		MerchantID:          merchantID,
		PassTypeIdentifier:  request.PassTypeIdentifier,
		ProgramName:         request.ProgramName,
		PassDescription:     request.PassDescription,
		StampGoal:           request.StampGoal,
		BackgroundColor:     request.BackgroundColor,
		ForegroundColor:     request.ForegroundColor,
		LabelColor:          request.LabelColor,
		AssetBundleRef:      request.AssetBundleRef,
		SupportText:         request.SupportText,
		TermsText:           request.TermsText,
		CampaignDisplayMode: request.CampaignDisplayMode,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, design)
}

func (c AppleWalletAdminController) authorize(w http.ResponseWriter, r *http.Request) bool {
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

func (c AppleWalletAdminController) route(path string) (string, string, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix)
	if rest == path {
		return "", "", false
	}
	parts := strings.Split(strings.Trim(rest, "/"), "/")
	if len(parts) < 2 || len(parts) > 3 || parts[0] == "" {
		return "", "", false
	}
	if parts[1] != "apple-wallet" && len(parts) != 2 {
		return "", "", false
	}
	return parts[0], strings.Join(parts[1:], "/"), true
}

func multipartFile(r *http.Request, field string, limit int64) ([]byte, error) {
	file, _, err := r.FormFile(field)
	if err != nil {
		return nil, fmt.Errorf("%s is required", field)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", field, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds the upload limit", field)
	}
	return data, nil
}
