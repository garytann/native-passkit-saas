package controller

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"native-ios-passkit-wip/internal/service"
)

type AdminAuthController struct {
	email    string
	password string
	tokens   service.TokenManager
}

func NewAdminAuthController(email string, password string, tokens service.TokenManager) AdminAuthController {
	return AdminAuthController{
		email:    strings.TrimSpace(strings.ToLower(email)),
		password: password,
		tokens:   tokens,
	}
}

type adminLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c AdminAuthController) Login(w http.ResponseWriter, r *http.Request) {
	if c.email == "" || c.password == "" {
		writeError(w, http.StatusServiceUnavailable, "platform admin login is not configured")
		return
	}
	var request adminLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	email := strings.TrimSpace(strings.ToLower(request.Email))
	validEmail := subtle.ConstantTimeCompare([]byte(email), []byte(c.email)) == 1
	validPassword := subtle.ConstantTimeCompare([]byte(request.Password), []byte(c.password)) == 1
	if !validEmail || !validPassword {
		writeError(w, http.StatusUnauthorized, "invalid administrator credentials")
		return
	}
	token, err := c.tokens.IssueAdminToken(email)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"admin": map[string]string{"email": email, "role": "platform_admin"},
		"token": token,
	})
}
