package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/service"
)

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

var errRouteNotFound = errors.New("pass route not found")

type HealthController struct{}

func (HealthController) Check(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

type CustomerController struct {
	members          service.MemberService
	tokens           service.TokenManager
	publicPathPrefix string
}

func NewCustomerController(members service.MemberService, tokens service.TokenManager, publicPathPrefix string) CustomerController {
	return CustomerController{
		members:          members,
		tokens:           tokens,
		publicPathPrefix: publicPathPrefix,
	}
}

type registerMemberRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type publicRegisterMemberRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type registerMemberResponse struct {
	Member     domain.StoreMember `json:"member"`
	PassURL    string             `json:"pass_url"`
	PreviewURL string             `json:"preview_url"`
	QRCodeURL  string             `json:"qr_code_url"`
}

type listMembersResponse struct {
	Members []domain.StoreMember `json:"members"`
}

type updateMemberStampsRequest struct {
	StampDelta int `json:"stamp_delta"`
}

type updateMemberStampsResponse struct {
	Member       domain.StoreMember           `json:"member"`
	Event        stampUpdateEvent             `json:"event"`
	WalletUpdate service.WalletUpdateDispatch `json:"wallet_update"`
}

type stampUpdateEvent struct {
	StampDelta            int `json:"stamp_delta"`
	PreviousStampProgress int `json:"previous_stamp_progress"`
	NewStampProgress      int `json:"new_stamp_progress"`
	StampGoal             int `json:"stamp_goal"`
}

type memberLookupResponse struct {
	Member      domain.StoreMember `json:"member"`
	StampGoal   int                `json:"stamp_goal"`
	RewardTitle string             `json:"reward_title"`
}

type memberRewardResponse struct {
	Member       domain.StoreMember             `json:"member"`
	Transaction  domain.MemberRewardTransaction `json:"transaction"`
	Replayed     bool                           `json:"replayed"`
	StampGoal    int                            `json:"stamp_goal"`
	RewardTitle  string                         `json:"reward_title"`
	WalletUpdate service.WalletUpdateDispatch   `json:"wallet_update"`
}

func (c CustomerController) MemberRoutes(w http.ResponseWriter, r *http.Request) {
	const prefix = "/api/v1/customers/members/"
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, prefix), "/")
	parts := strings.Split(path, "/")

	switch {
	case len(parts) == 1 && parts[0] != "" && r.Method == http.MethodGet:
		c.GetMember(w, r, parts[0])
	case len(parts) == 2 && parts[0] != "" && parts[1] == "stamps" && r.Method == http.MethodPut:
		c.UpdateMemberStamps(w, r)
	case len(parts) == 3 && parts[0] != "" && parts[1] == "rewards" && parts[2] == "issue" && r.Method == http.MethodPost:
		c.MutateMemberReward(w, r, parts[0], "issue")
	case len(parts) == 3 && parts[0] != "" && parts[1] == "rewards" && parts[2] == "redeem" && r.Method == http.MethodPost:
		c.MutateMemberReward(w, r, parts[0], "redeem")
	default:
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
	}
}

func (c CustomerController) GetMember(w http.ResponseWriter, r *http.Request, memberID string) {
	claims, err := c.merchantClaims(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	result, err := c.members.GetMemberForMerchant(r.Context(), memberID, claims.Merchant)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, memberLookupResponse{
		Member: result.Member, StampGoal: result.StampGoal, RewardTitle: result.RewardTitle,
	})
}

func (c CustomerController) MutateMemberReward(w http.ResponseWriter, r *http.Request, memberID string, operation string) {
	claims, err := c.merchantClaims(r)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	input := service.MutateMemberRewardInput{
		MemberID:       memberID,
		MerchantID:     claims.MerchantID,
		Merchant:       claims.Merchant,
		IdempotencyKey: r.Header.Get("Idempotency-Key"),
	}
	var result service.MutateMemberRewardResult
	if operation == "issue" {
		result, err = c.members.IssueMemberReward(r.Context(), input)
	} else {
		result, err = c.members.RedeemMemberReward(r.Context(), input)
	}
	if err != nil {
		writeServiceError(w, err)
		return
	}
	status := http.StatusCreated
	if result.Replayed {
		status = http.StatusOK
	}
	writeJSON(w, status, memberRewardResponse{
		Member:       result.Member,
		Transaction:  result.Transaction,
		Replayed:     result.Replayed,
		StampGoal:    result.StampGoal,
		RewardTitle:  result.RewardTitle,
		WalletUpdate: result.WalletUpdate,
	})
}

func (c CustomerController) merchantClaims(r *http.Request) (service.MerchantTokenClaims, error) {
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		return service.MerchantTokenClaims{}, err
	}
	return c.tokens.VerifyMerchantToken(token)
}

func (c CustomerController) ListMembers(w http.ResponseWriter, r *http.Request) {
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	claims, err := c.tokens.VerifyMerchantToken(token)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	members, err := c.members.ListMembersForMerchant(r.Context(), claims.Merchant)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, listMembersResponse{Members: members})
}

func (c CustomerController) RegisterMember(w http.ResponseWriter, r *http.Request) {
	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	claims, err := c.tokens.VerifyMerchantToken(token)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	var request registerMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := c.members.RegisterMember(r.Context(), service.RegisterMemberInput{
		Name:     request.Name,
		Email:    request.Email,
		Merchant: claims.Merchant,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMemberResponse{
		Member:     result.Member,
		PassURL:    result.PassURL,
		PreviewURL: result.PreviewURL,
		QRCodeURL:  result.QRCodeURL,
	})
}

func (c CustomerController) UpdateMemberStamps(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromStampPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	token, err := service.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	claims, err := c.tokens.VerifyMerchantToken(token)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	var request updateMemberStampsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	update, err := c.members.UpdateMemberStamps(r.Context(), service.UpdateMemberStampsInput{
		MemberID:   memberID,
		Merchant:   claims.Merchant,
		StampDelta: request.StampDelta,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, updateMemberStampsResponse{
		Member: update.Member,
		Event: stampUpdateEvent{
			StampDelta:            update.StampDelta,
			PreviousStampProgress: update.PreviousStampProgress,
			NewStampProgress:      update.NewStampProgress,
			StampGoal:             update.StampGoal,
		},
		WalletUpdate: update.WalletUpdate,
	})
}

func (c CustomerController) memberIDFromStampPath(path string) (string, bool) {
	const prefix = "/api/v1/customers/members/"
	const suffix = "/stamps"
	if !hasRouteShape(path, prefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, prefix, suffix), true
}

func (c CustomerController) RegisterPublicMember(w http.ResponseWriter, r *http.Request) {
	merchantSlug, ok := c.merchantSlugFromPublicMemberPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	var request publicRegisterMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := c.members.RegisterMemberByMerchantSlug(r.Context(), merchantSlug, service.RegisterMemberInput{
		Name:  request.Name,
		Email: request.Email,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMemberResponse{
		Member:     result.Member,
		PassURL:    result.PassURL,
		PreviewURL: result.PreviewURL,
		QRCodeURL:  result.QRCodeURL,
	})
}

func (c CustomerController) merchantSlugFromPublicMemberPath(path string) (string, bool) {
	const suffix = "/members/register"
	if !hasRouteShape(path, c.publicPathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.publicPathPrefix, suffix), true
}

type MerchantController struct {
	merchants service.MerchantService
}

func NewMerchantController(merchants service.MerchantService) MerchantController {
	return MerchantController{merchants: merchants}
}

type registerMerchantRequest struct {
	FirstName     string `json:"first_name"`
	LastName      string `json:"last_name"`
	Company       string `json:"company"`
	MerchantEmail string `json:"merchant_email"`
	Password      string `json:"password"`
}

type registerMerchantResponse struct {
	Merchant domain.Merchant `json:"merchant"`
}

type loginMerchantRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginMerchantResponse struct {
	Merchant domain.Merchant `json:"merchant"`
	Token    string          `json:"token"`
}

func (c MerchantController) RegisterMerchant(w http.ResponseWriter, r *http.Request) {
	var request registerMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	merchant, err := c.merchants.RegisterMerchant(r.Context(), service.RegisterMerchantInput{
		FirstName:     request.FirstName,
		LastName:      request.LastName,
		Company:       request.Company,
		MerchantEmail: request.MerchantEmail,
		Password:      request.Password,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, registerMerchantResponse{
		Merchant: merchant,
	})
}

func (c MerchantController) LoginMerchant(w http.ResponseWriter, r *http.Request) {
	var request loginMerchantRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	result, err := c.merchants.LoginMerchant(r.Context(), service.LoginMerchantInput{
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, loginMerchantResponse{
		Merchant: result.Merchant,
		Token:    result.Token,
	})
}

type PassController struct {
	passes     service.PassService
	wallet     service.WalletService
	pathPrefix string
}

func NewPassController(passes service.PassService, wallet service.WalletService, pathPrefix string) PassController {
	return PassController{
		passes:     passes,
		wallet:     wallet,
		pathPrefix: pathPrefix,
	}
}

type registerWalletPassRequest struct {
	PushToken string `json:"pushToken"`
}

func (c PassController) Example(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, c.passes.ExampleLinks("member-1001"))
}

func (c PassController) Routes(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, c.pathPrefix+"v1/") {
		c.WalletWebService(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, ok := c.memberIDFromPreviewPath(r.URL.Path); ok {
		c.Preview(w, r)
		return
	}
	if _, ok := c.memberIDFromQRCodePath(r.URL.Path); ok {
		c.QRCode(w, r)
		return
	}
	c.Package(w, r)
}

func (c PassController) WalletWebService(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == c.pathPrefix+"v1/log" && r.Method == http.MethodPost {
		w.WriteHeader(http.StatusOK)
		return
	}

	if params, ok := c.walletRegistrationPath(r.URL.Path); ok {
		switch r.Method {
		case http.MethodPost:
			c.RegisterWalletPass(w, r, params)
		case http.MethodDelete:
			c.UnregisterWalletPass(w, r, params)
		default:
			w.Header().Set("Allow", "POST, DELETE")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if params, ok := c.walletUpdatedSerialsPath(r.URL.Path); ok {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		c.UpdatedSerialNumbers(w, r, params)
		return
	}

	if params, ok := c.walletPassPath(r.URL.Path); ok {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		c.UpdatedPackage(w, r, params)
		return
	}

	writeError(w, http.StatusNotFound, errRouteNotFound.Error())
}

func (c PassController) RegisterWalletPass(w http.ResponseWriter, r *http.Request, params walletPathParams) {
	authorization := r.Header.Get("Authorization")
	if err := c.passes.AuthorizePass(r.Context(), params.PassTypeIdentifier, params.SerialNumber, authorization); err != nil {
		writeServiceError(w, err)
		return
	}

	var request registerWalletPassRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	created, err := c.wallet.RegisterPass(r.Context(), service.RegisterWalletPassInput{
		DeviceLibraryIdentifier: params.DeviceLibraryIdentifier,
		PassTypeIdentifier:      params.PassTypeIdentifier,
		SerialNumber:            params.SerialNumber,
		PushToken:               request.PushToken,
		AuthenticationToken:     applePassAuthorizationToken(authorization),
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	if created {
		w.WriteHeader(http.StatusCreated)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c PassController) UnregisterWalletPass(w http.ResponseWriter, r *http.Request, params walletPathParams) {
	if err := c.passes.AuthorizePass(r.Context(), params.PassTypeIdentifier, params.SerialNumber, r.Header.Get("Authorization")); err != nil {
		writeServiceError(w, err)
		return
	}
	if err := c.wallet.UnregisterPass(r.Context(), params.DeviceLibraryIdentifier, params.PassTypeIdentifier, params.SerialNumber); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (c PassController) UpdatedSerialNumbers(w http.ResponseWriter, r *http.Request, params walletPathParams) {
	result, err := c.wallet.UpdatedSerialNumbers(r.Context(), params.DeviceLibraryIdentifier, params.PassTypeIdentifier, r.URL.Query().Get("passesUpdatedSince"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if len(result.SerialNumbers) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (c PassController) UpdatedPackage(w http.ResponseWriter, r *http.Request, params walletPathParams) {
	data, err := c.passes.AuthenticatedPackage(r.Context(), params.PassTypeIdentifier, params.SerialNumber, r.Header.Get("Authorization"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.pkpass")
	w.Header().Set("Content-Disposition", `attachment; filename="`+params.SerialNumber+`.pkpass"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (c PassController) Package(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromPackagePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	data, err := c.passes.CreatePackage(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/vnd.apple.pkpass")
	w.Header().Set("Content-Disposition", `attachment; filename="`+memberID+`.pkpass"`)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (c PassController) Preview(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromPreviewPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	data, err := c.passes.Preview(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	templateData := previewPageData{
		PassPreview: data,
		PassColorDeclarations: template.CSS(
			"background: " + string(safePreviewCSSColor(data.Pass.BackgroundColor, "rgb(84, 52, 245)")) + "; " +
				"color: " + string(safePreviewCSSColor(data.Pass.ForegroundColor, "rgb(255, 255, 255)")) + ";",
		),
		LabelColorDeclaration: template.CSS(
			"color: " + string(safePreviewCSSColor(data.Pass.LabelColor, "rgb(225, 220, 255)")) + ";",
		),
	}
	var out bytes.Buffer
	if err := previewTemplate.Execute(&out, templateData); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Bytes())
}

type previewPageData struct {
	service.PassPreview
	PassColorDeclarations template.CSS
	LabelColorDeclaration template.CSS
}

var previewCSSColorPattern = regexp.MustCompile(`^(?:#[0-9A-Fa-f]{6}|rgb\([0-9]{1,3}, [0-9]{1,3}, [0-9]{1,3}\))$`)

func safePreviewCSSColor(value string, fallback string) template.CSS {
	if !previewCSSColorPattern.MatchString(value) {
		value = fallback
	}
	return template.CSS(value)
}

func (c PassController) QRCode(w http.ResponseWriter, r *http.Request) {
	memberID, ok := c.memberIDFromQRCodePath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errRouteNotFound.Error())
		return
	}

	png, err := c.passes.QRCode(r.Context(), memberID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(png)
}

func (c PassController) memberIDFromPackagePath(path string) (string, bool) {
	const suffix = ".pkpass"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

func (c PassController) memberIDFromPreviewPath(path string) (string, bool) {
	const suffix = "/preview"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

func (c PassController) memberIDFromQRCodePath(path string) (string, bool) {
	const suffix = "/qr.png"
	if !hasRouteShape(path, c.pathPrefix, suffix) {
		return "", false
	}
	return trimRouteValue(path, c.pathPrefix, suffix), true
}

type walletPathParams struct {
	DeviceLibraryIdentifier string
	PassTypeIdentifier      string
	SerialNumber            string
}

func (c PassController) walletRegistrationPath(path string) (walletPathParams, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix+"v1/")
	parts := strings.Split(rest, "/")
	if len(parts) != 5 || parts[0] != "devices" || parts[2] != "registrations" {
		return walletPathParams{}, false
	}
	return walletPathParams{
		DeviceLibraryIdentifier: parts[1],
		PassTypeIdentifier:      parts[3],
		SerialNumber:            parts[4],
	}, true
}

func (c PassController) walletUpdatedSerialsPath(path string) (walletPathParams, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix+"v1/")
	parts := strings.Split(rest, "/")
	if len(parts) != 4 || parts[0] != "devices" || parts[2] != "registrations" {
		return walletPathParams{}, false
	}
	return walletPathParams{
		DeviceLibraryIdentifier: parts[1],
		PassTypeIdentifier:      parts[3],
	}, true
}

func (c PassController) walletPassPath(path string) (walletPathParams, bool) {
	rest := strings.TrimPrefix(path, c.pathPrefix+"v1/")
	parts := strings.Split(rest, "/")
	if len(parts) != 3 || parts[0] != "passes" {
		return walletPathParams{}, false
	}
	return walletPathParams{
		PassTypeIdentifier: parts[1],
		SerialNumber:       parts[2],
	}, true
}

func applePassAuthorizationToken(authorization string) string {
	scheme, token, ok := strings.Cut(strings.TrimSpace(authorization), " ")
	if !ok || scheme != "ApplePass" {
		return ""
	}
	return strings.TrimSpace(token)
}

func hasRouteShape(path string, prefix string, suffix string) bool {
	return strings.HasPrefix(path, prefix) && strings.HasSuffix(path, suffix)
}

func trimRouteValue(path string, prefix string, suffix string) string {
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.TrimSpace(value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validationErr service.ValidationError
	switch {
	case errors.Is(err, service.ErrRepositoryUnavailable):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, service.ErrUnauthorized):
		writeError(w, http.StatusUnauthorized, "unauthorized")
	case errors.Is(err, service.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden")
	case errors.As(err, &validationErr):
		writeError(w, http.StatusBadRequest, validationErr.Error())
	case errors.Is(err, domain.ErrMerchantAlreadyExists):
		writeError(w, http.StatusConflict, "merchant or merchant_email already exists")
	case errors.Is(err, domain.ErrInvalidMerchantCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, domain.ErrMerchantNotFound):
		writeError(w, http.StatusNotFound, "merchant not found")
	case errors.Is(err, domain.ErrAppleWalletIssuerNotFound):
		writeError(w, http.StatusNotFound, "apple wallet issuer not found")
	case errors.Is(err, domain.ErrAppleWalletPassTypeNotFound):
		writeError(w, http.StatusNotFound, "apple wallet pass type not found")
	case errors.Is(err, domain.ErrAppleWalletCertificateNotFound):
		writeError(w, http.StatusNotFound, "apple wallet certificate not found")
	case errors.Is(err, domain.ErrApplePassDesignNotFound):
		writeError(w, http.StatusNotFound, "apple pass design not found")
	case errors.Is(err, domain.ErrAppleWalletNotProvisioned):
		writeError(w, http.StatusConflict, "apple wallet is not provisioned for merchant")
	case errors.Is(err, domain.ErrAppleWalletConfigurationConflict):
		writeError(w, http.StatusConflict, "apple wallet configuration already exists")
	case errors.Is(err, domain.ErrStoreMemberNotFound):
		writeError(w, http.StatusNotFound, "member not found")
	case errors.Is(err, domain.ErrMemberInactive):
		writeError(w, http.StatusConflict, "member is inactive")
	case errors.Is(err, domain.ErrRewardNotReady):
		writeError(w, http.StatusConflict, "member has not completed the stamp goal")
	case errors.Is(err, domain.ErrNoAvailableRewards):
		writeError(w, http.StatusConflict, "member has no available rewards")
	case errors.Is(err, domain.ErrIdempotencyKeyConflict):
		writeError(w, http.StatusConflict, "idempotency key was already used for another operation")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

var previewTemplate = template.Must(template.New("pass-preview").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.Member.ID}} Apple Wallet Preview</title>
  <style>
    :root {
      color-scheme: light;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #f4f6f8;
      color: #12202f;
    }
    body {
      margin: 0;
      min-height: 100vh;
      display: grid;
      place-items: center;
      padding: 32px 16px;
    }
    main {
      width: min(420px, 100%);
    }
    .pass {
      {{.PassColorDeclarations}}
      border-radius: 18px;
      overflow: hidden;
      box-shadow: 0 22px 50px rgba(20, 35, 54, .24);
    }
    .top {
      display: flex;
      justify-content: space-between;
      gap: 16px;
      padding: 22px 24px 12px;
      align-items: start;
    }
    .logo {
      width: 160px;
      height: 50px;
      border-radius: 0;
      flex: 0 1 52%;
      object-fit: contain;
      object-position: left center;
    }
    .logo-placeholder {
      background: #fff;
    }
    .label {
      {{.LabelColorDeclaration}}
      font-size: 10px;
      line-height: 1.2;
      text-transform: uppercase;
      font-weight: 700;
      letter-spacing: .04em;
    }
    .value {
      font-size: 18px;
      font-weight: 700;
      line-height: 1.2;
      margin-top: 3px;
    }
	.strip-area {
	  position: relative;
	  min-height: 144px;
	  overflow: hidden;
	}
	.strip-artwork {
	  position: absolute;
	  inset: 0;
	  width: 100%;
	  height: 144px;
	  object-fit: cover;
	}
    .supporting-fields {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
      gap: 18px;
      padding: 0 24px 22px;
    }
    .supporting-fields .value {
      font-size: 18px;
      font-weight: 700;
    }
    .barcode {
      background: #fff;
      color: #1d1d1f;
      margin: 0 24px 24px;
      border-radius: 10px;
      padding: 18px;
      text-align: center;
    }
    .qr {
      width: 176px;
      height: 176px;
      margin: 0 auto 10px;
      display: block;
    }
    .alt {
      font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
      font-size: 13px;
      overflow-wrap: anywhere;
    }
    .meta {
      margin-top: 18px;
      font-size: 13px;
      line-height: 1.5;
      color: #4a5563;
    }
    .meta a {
      color: #195da8;
      overflow-wrap: anywhere;
    }
    .information {
      margin-top: 18px;
      padding: 20px 22px;
      border-radius: 18px;
      background: #fff;
      box-shadow: 0 14px 34px rgba(20, 35, 54, .12);
    }
    .information h2 {
      margin: 0 0 16px;
      font-size: 18px;
    }
    .info-field {
      padding: 12px 0;
      border-top: 1px solid #e7eaf0;
    }
    .info-field:first-of-type {
      border-top: 0;
      padding-top: 0;
    }
    .info-field .value {
      color: #313a47;
      font-size: 14px;
      font-weight: 500;
      line-height: 1.45;
      overflow-wrap: anywhere;
    }
  </style>
</head>
<body>
  <main>
    <section class="pass" aria-label="Apple Wallet pass preview">
      <div class="top">
        {{if .LogoBase64}}
        <img class="logo" src="data:image/png;base64,{{.LogoBase64}}" alt="Merchant logo">
        {{else}}
        <div class="logo logo-placeholder" aria-label="Merchant logo placeholder"></div>
        {{end}}
      </div>
	  {{if .StripBase64}}
	  <div class="strip-area">
		<img class="strip-artwork" src="data:image/png;base64,{{.StripBase64}}" alt="Stamp progress">
	  </div>
	  {{end}}
      <div class="supporting-fields">
        {{with index .Pass.StoreCard.SecondaryFields 0}}
        <div class="secondary">
          <div class="label">{{.Label}}</div>
          <div class="value">{{.Value}}</div>
        </div>
        {{end}}
        {{range .Pass.StoreCard.AuxiliaryFields}}
        <div class="auxiliary">
          <div class="label">{{.Label}}</div>
          <div class="value">{{.Value}}</div>
        </div>
        {{end}}
      </div>
      <div class="barcode">
        <img class="qr" src="{{.QRCodeURL}}" alt="QR code for {{.Barcode.AltText}}">
        <div class="alt">{{.Barcode.AltText}}</div>
      </div>
    </section>
    <section class="information" aria-label="Apple Wallet pass information">
      <h2>Pass information</h2>
      {{range .Pass.StoreCard.BackFields}}
      <div class="info-field">
        <div class="label">{{.Label}}</div>
        <div class="value">{{.Value}}</div>
      </div>
      {{end}}
    </section>
    <div class="meta">
      Preview only. Apple Wallet performs the final native rendering after a signed pass is opened on device.
      <br>
      Pass URL: <a href="{{.PassURL}}">{{.PassURL}}</a>
    </div>
  </main>
</body>
</html>`))
