package controller

import (
	"encoding/json"
	"net/http"

	"native-ios-passkit-wip/internal/service"
)

type MerchantPassDesignController struct {
	passes service.MerchantPassDesignService
	tokens service.TokenManager
}

type publishMerchantPassDesignRequest struct {
	ProgramName         string                         `json:"program_name"`
	PassDescription     string                         `json:"pass_description"`
	StampGoal           int                            `json:"stamp_goal"`
	StampInstructions   string                         `json:"stamp_instructions"`
	RewardTitle         string                         `json:"reward_title"`
	RewardDetails       string                         `json:"reward_details"`
	SupportText         string                         `json:"support_text"`
	TermsText           string                         `json:"terms_text"`
	Links               []service.ApplePassLinkRequest `json:"links"`
	CampaignDisplayMode string                         `json:"campaign_display_mode"`
}

func NewMerchantPassDesignController(passes service.MerchantPassDesignService, tokens service.TokenManager) MerchantPassDesignController {
	return MerchantPassDesignController{passes: passes, tokens: tokens}
}

func (c MerchantPassDesignController) Routes(w http.ResponseWriter, r *http.Request) {
	claims, ok := c.merchantClaims(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		result, err := c.passes.Design(r.Context(), claims.MerchantID)
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	case http.MethodPut:
		var request publishMerchantPassDesignRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
		result, err := c.passes.Publish(r.Context(), service.PublishMerchantPassDesignRequest{
			MerchantID:          claims.MerchantID,
			ProgramName:         request.ProgramName,
			PassDescription:     request.PassDescription,
			StampGoal:           request.StampGoal,
			StampInstructions:   request.StampInstructions,
			RewardTitle:         request.RewardTitle,
			RewardDetails:       request.RewardDetails,
			SupportText:         request.SupportText,
			TermsText:           request.TermsText,
			Links:               request.Links,
			CampaignDisplayMode: request.CampaignDisplayMode,
		})
		if err != nil {
			writeServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (c MerchantPassDesignController) merchantClaims(w http.ResponseWriter, r *http.Request) (service.MerchantTokenClaims, bool) {
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
