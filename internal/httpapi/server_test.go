package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"native-ios-passkit-wip/internal/config"
)

func TestRoutesUseAPIV1Prefix(t *testing.T) {
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil)
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
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil)
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
	server := NewServer(config.Config{PublicBaseURL: "http://localhost:8080"}, nil)
	routes := server.Routes()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/customers/members/register",
		strings.NewReader(`{"name":"Test","email":"test@example.com","merchant":"Saga"}`),
	)
	request.Header.Set("Content-Type", "application/json")
	routes.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected versioned customer registration route to return 503 without db, got %d", response.Code)
	}
}
