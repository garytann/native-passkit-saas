package customer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"native-ios-passkit-wip/internal/domain"
)

type Repository interface {
	Create(ctx context.Context, input domain.CreateStoreMemberInput) (domain.StoreMember, error)
}

type Handler struct {
	repo       Repository
	apiBaseURL string
}

func NewHandler(repo Repository, apiBaseURL string) Handler {
	return Handler{
		repo:       repo,
		apiBaseURL: strings.TrimRight(apiBaseURL, "/"),
	}
}

type registerMemberRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Merchant string `json:"merchant"`
}

type registerMemberResponse struct {
	Member     domain.StoreMember `json:"member"`
	PassURL    string             `json:"pass_url"`
	PreviewURL string             `json:"preview_url"`
	QRCodeURL  string             `json:"qr_code_url"`
}

func (h Handler) RegisterMember(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeError(w, http.StatusServiceUnavailable, errors.New("database is not configured"))
		return
	}

	var request registerMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON request body"))
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Email = strings.TrimSpace(strings.ToLower(request.Email))
	request.Merchant = strings.TrimSpace(request.Merchant)
	if err := validateRegisterMemberRequest(request); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	memberID, err := domain.NewReadableMemberID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	member, err := h.repo.Create(ctx, domain.CreateStoreMemberInput{
		MemberID:      memberID,
		CustomerName:  request.Name,
		CustomerEmail: request.Email,
		Merchant:      request.Merchant,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMemberResponse{
		Member:     member,
		PassURL:    h.passURL(member.MemberID),
		PreviewURL: h.previewURL(member.MemberID),
		QRCodeURL:  h.qrCodeURL(member.MemberID),
	})
}

func validateRegisterMemberRequest(request registerMemberRequest) error {
	switch {
	case request.Name == "":
		return errors.New("name is required")
	case request.Email == "":
		return errors.New("email is required")
	case !strings.Contains(request.Email, "@"):
		return errors.New("email must be valid")
	case request.Merchant == "":
		return errors.New("merchant is required")
	default:
		return nil
	}
}

func (h Handler) passURL(memberID string) string {
	return h.apiBaseURL + "/apple/passes/" + memberID + ".pkpass"
}

func (h Handler) previewURL(memberID string) string {
	return h.apiBaseURL + "/apple/passes/" + memberID + "/preview"
}

func (h Handler) qrCodeURL(memberID string) string {
	return h.apiBaseURL + "/apple/passes/" + memberID + "/qr.png"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
