package controller

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"native-ios-passkit-wip/internal/service"
)

type MerchantPassDesignController struct {
	passes service.MerchantPassDesignService
	tokens service.TokenManager
}

func (c MerchantPassDesignController) StampIcon(w http.ResponseWriter, r *http.Request) {
	claims, ok := c.merchantClaims(w, r)
	if !ok {
		return
	}
	if r.Method == http.MethodGet {
		data, err := c.passes.StampArtwork(r.Context(), claims.MerchantID, r.URL.Query().Get("source"))
		if err != nil {
			writeServiceError(w, err)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "private, max-age=300")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
		return
	}
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "GET, PUT")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := r.ParseMultipartForm(2 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "stamp icon update must be multipart form data")
		return
	}
	iconKey := r.FormValue("stamp_icon_key")
	var iconPNG []byte
	if iconKey == "custom" {
		var err error
		iconPNG, err = merchantMultipartFile(r, "stamp_icon", maxStampIconUploadSize)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	result, err := c.passes.PublishStampIcon(r.Context(), service.PublishMerchantStampIconRequest{
		MerchantID: claims.MerchantID,
		IconKey:    iconKey,
		IconPNG:    iconPNG,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

const maxStampIconUploadSize int64 = 1 << 20

func merchantMultipartFile(r *http.Request, field string, limit int64) ([]byte, error) {
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
	StampIconKey        string                         `json:"stamp_icon_key"`
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
			StampIconKey:        request.StampIconKey,
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
