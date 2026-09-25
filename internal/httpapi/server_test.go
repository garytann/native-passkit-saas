package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"native-ios-passkit-wip/internal/config"
	"native-ios-passkit-wip/internal/domain"
	"native-ios-passkit-wip/internal/service"
)

type capturingMemberRepository struct {
	createdInput domain.CreateStoreMemberInput
}

func (r *capturingMemberRepository) Create(_ context.Context, input domain.CreateStoreMemberInput) (domain.StoreMember, error) {
	r.createdInput = input
	return domain.StoreMember{
		MemberID:      input.MemberID,
		CustomerName:  input.CustomerName,
		CustomerEmail: input.CustomerEmail,
		Merchant:      input.Merchant,
		Status:        "active",
	}, nil
}

func (*capturingMemberRepository) GetByID(context.Context, string) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (*capturingMemberRepository) GetByIDForMerchant(context.Context, string, string) (domain.StoreMember, error) {
	return domain.StoreMember{}, nil
}

func (*capturingMemberRepository) ListByMerchant(context.Context, string) ([]domain.StoreMember, error) {
	return nil, nil
}

func (*capturingMemberRepository) UpdateStampProgressForMerchant(context.Context, string, string, int, int) (domain.StampUpdate, error) {
	return domain.StampUpdate{}, nil
}

func (*capturingMemberRepository) ApplyRewardMutation(context.Context, domain.RewardMutationInput) (domain.RewardMutation, error) {
	return domain.RewardMutation{}, nil
}

func TestRoutesUseAPIV1Prefix(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/healthz", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected versioned health route to return 200, got %d", response.Code)
	}

	response = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected unversioned health route to return 404, got %d", response.Code)
	}
}

func TestPassExampleReturnsVersionedURLs(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/apple/passes/example", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("expected pass example route to return 200, got %d", response.Code)
	}

	body := response.Body.String()
	if !strings.Contains(body, "http://localhost:8080/api/v1/apple/passes/member-1001.pkpass") {
		t.Fatalf("expected versioned pass url in response, got %s", body)
	}
	if !strings.Contains(body, "http://localhost:8080/api/v1/apple/passes/member-1001/preview") {
		t.Fatalf("expected versioned preview url in response, got %s", body)
	}
}

func TestCustomerRegistrationRequiresMerchantToken(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/customers/members/register",
		strings.NewReader(`{"name":"Test","email":"test@example.com","merchant":"Just"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected customer registration route to require auth token, got %d", response.Code)
	}
}

func TestCustomerRegistrationUsesMerchantFromToken(t *testing.T) {
	const secret = "customer-registration-test-secret"
	repository := &capturingMemberRepository{}
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080", AuthTokenSecret: secret}, repository, nil)
	routes := server.Routes()
	token, err := service.NewTokenManager(secret).IssueMerchantToken("MCH-REAL1234", "Registered Coffee")
	if err != nil {
		t.Fatalf("issue merchant token: %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/customers/members/register",
		strings.NewReader(`{"name":"Taylor","email":"taylor@example.com","merchant":"Spoofed Merchant"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)
	routes.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected customer registration to return 201, got %d: %s", response.Code, response.Body.String())
	}
	if repository.createdInput.Merchant != "Registered Coffee" {
		t.Fatalf("expected token merchant to be stored, got %q", repository.createdInput.Merchant)
	}
}

func TestMerchantRegistrationRouteIsVersioned(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/merchants/register",
		strings.NewReader(`{"first_name":"Gary","last_name":"Lee","company":"Just Loyalty","merchant_email":"merchant@example.com","password":"password123"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected versioned merchant registration route to return 503 without db, got %d", response.Code)
	}
}

func TestMerchantLoginRouteIsVersioned(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/merchants/login",
		strings.NewReader(`{"email":"merchant@example.com","password":"password123"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected versioned merchant login route to return 503 without db, got %d", response.Code)
	}
}

func TestMembersListRequiresMerchantToken(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/customers/members", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected members list route to require auth token, got %d", response.Code)
	}
}

func TestPublicMerchantMemberRegistrationRouteIsVersioned(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/public/merchants/daily-brew/members/register",
		strings.NewReader(`{"name":"Taylor","email":"taylor@example.com"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected public merchant member registration route to return 503 without db, got %d", response.Code)
	}
}

func TestUpdateMemberStampsRequiresMerchantToken(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/customers/members/MBR-ABC12345/stamps",
		strings.NewReader(`{"stamp_delta":1}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected stamp update route to require auth token, got %d", response.Code)
	}
}

func TestMerchantPassDesignRequiresMerchantToken(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/me/apple-wallet/pass-design", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected merchant pass design route to require auth token, got %d", response.Code)
	}
}

func TestMerchantPassDesignUsesAuthenticatedMerchant(t *testing.T) {
	const secret = "merchant-pass-design-test-secret"
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080", AuthTokenSecret: secret}, nil, nil)
	routes := server.Routes()
	token, err := service.NewTokenManager(secret).IssueMerchantToken("MCH-REAL1234", "Registered Coffee")
	if err != nil {
		t.Fatalf("issue merchant token: %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/merchants/me/apple-wallet/pass-design", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected authenticated request to reach unavailable repository, got %d: %s", response.Code, response.Body.String())
	}
}

func TestAppleWalletAdminRouteRequiresAdminToken(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080", AdminAPIToken: "admin-secret"}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/merchants/MCH-ABC12345/apple-wallet", nil)
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected Apple Wallet admin route to require admin token, got %d", response.Code)
	}
}

func TestPlatformAdminLoginIssuesAuthorizedToken(t *testing.T) {
	server := NewServer(config.Config{
		PublicBaseURL:         "http://localhost:8080",
		AuthTokenSecret:       "test-signing-secret",
		PlatformAdminEmail:    "admin@justloyalty.test",
		PlatformAdminPassword: "correct-password",
	}, nil, nil)
	routes := server.Routes()

	loginResponse := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{"email":"admin@justloyalty.test","password":"correct-password"}`))
	loginRequest.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("expected admin login to return 200, got %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	var login struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &login); err != nil || login.Token == "" {
		t.Fatalf("expected admin token in response: %v", err)
	}

	listResponse := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/merchants", nil)
	listRequest.Header.Set("Authorization", "Bearer "+login.Token)
	routes.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected authorized request to reach unavailable repository, got %d", listResponse.Code)
	}
}

func TestPlatformAdminLoginRejectsInvalidPassword(t *testing.T) {
	server := NewServer(config.Config{
		PublicBaseURL:         "http://localhost:8080",
		AuthTokenSecret:       "test-signing-secret",
		PlatformAdminEmail:    "admin@justloyalty.test",
		PlatformAdminPassword: "correct-password",
	}, nil, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/auth/login", strings.NewReader(`{"email":"admin@justloyalty.test","password":"wrong-password"}`))
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected invalid admin login to return 401, got %d", response.Code)
	}
}
