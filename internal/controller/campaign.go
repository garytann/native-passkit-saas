package controller

import (
	"encoding/json"
	"net/http"

	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/service"
)

type CampaignController struct {
	campaigns service.CampaignService
	tokens    service.TokenManager
}

func NewCampaignController(campaigns service.CampaignService, tokens service.TokenManager) CampaignController {
	return CampaignController{
		campaigns: campaigns,
		tokens:    tokens,
	}
}

func (c CampaignController) Routes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		c.ListCampaigns(w, r)
	case http.MethodPost:
		c.CreateCampaign(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type createCampaignRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

type listCampaignsResponse struct {
	Campaigns []domain.Campaign `json:"campaigns"`
}

func (c CampaignController) ListCampaigns(w http.ResponseWriter, r *http.Request) {
	claims, ok := c.merchantClaims(w, r)
	if !ok {
		return
	}

	campaigns, err := c.campaigns.ListForMerchant(r.Context(), claims.Merchant)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, listCampaignsResponse{Campaigns: campaigns})
}

func (c CampaignController) CreateCampaign(w http.ResponseWriter, r *http.Request) {
	claims, ok := c.merchantClaims(w, r)
	if !ok {
		return
	}

	var request createCampaignRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := c.campaigns.CreateForMerchant(r.Context(), service.CreateCampaignRequest{
		Merchant: claims.Merchant,
		Title:    request.Title,
		Message:  request.Message,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

func (c CampaignController) merchantClaims(w http.ResponseWriter, r *http.Request) (service.MerchantTokenClaims, bool) {
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeServiceError(w, err)
		return service.MerchantTokenClaims{}, false
	}

	claims, err := c.tokens.VerifyMerchantToken(token)
	if err != nil {
		writeServiceError(w, err)
		return service.MerchantTokenClaims{}, false
	}

	return claims, true
}
