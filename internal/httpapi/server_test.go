package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"native-ios-passkit-wip/internal/config"
)

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

func TestCustomerRegistrationRouteIsVersioned(t *testing.T) {
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
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected versioned customer registration route to return 503 without db, got %d", response.Code)
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
