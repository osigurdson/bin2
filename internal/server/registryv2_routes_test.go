package server

import (
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistryV2RootRequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := newRegistryV2RootTestServer(t)

	tests := []struct {
		name   string
		method string
	}{
		{name: "get", method: http.MethodGet},
		{name: "head", method: http.MethodHead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://registry.test/v2/", nil)
			res := httptest.NewRecorder()

			s.router.ServeHTTP(res, req)

			if res.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
			}
			if got := res.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
				t.Fatalf("Docker-Distribution-API-Version = %q", got)
			}
			if got := res.Header().Get("WWW-Authenticate"); got != `Bearer realm="http://registry.test/v2/token",service="registry.test"` {
				t.Fatalf("WWW-Authenticate = %q", got)
			}
			if tt.method == http.MethodHead && res.Body.Len() != 0 {
				t.Fatalf("HEAD body len = %d, want 0", res.Body.Len())
			}
		})
	}
}

func TestRegistryV2RootAcceptsBearerAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := newRegistryV2RootTestServer(t)
	token, _, _, err := s.issueRegistryToken("alpha", "registry.test", nil)
	if err != nil {
		t.Fatalf("issueRegistryToken: %v", err)
	}

	tests := []struct {
		name   string
		method string
	}{
		{name: "get", method: http.MethodGet},
		{name: "head", method: http.MethodHead},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://registry.test/v2/", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			res := httptest.NewRecorder()

			s.router.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
			}
			if got := res.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
				t.Fatalf("Docker-Distribution-API-Version = %q", got)
			}
			if got := res.Header().Get("WWW-Authenticate"); got != "" {
				t.Fatalf("WWW-Authenticate = %q, want empty", got)
			}
			if res.Body.Len() != 0 {
				t.Fatalf("body len = %d, want 0", res.Body.Len())
			}
		})
	}
}

func TestRegistryV2RootRejectsUnsupportedMethod(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := newRegistryV2RootTestServer(t)
	token, _, _, err := s.issueRegistryToken("alpha", "registry.test", nil)
	if err != nil {
		t.Fatalf("issueRegistryToken: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://registry.test/v2/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	s.router.ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusMethodNotAllowed)
	}
	if got := res.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
		t.Fatalf("Docker-Distribution-API-Version = %q", got)
	}
	if got := res.Header().Get("Content-Type"); !strings.HasPrefix(got, "application/json") {
		t.Fatalf("Content-Type = %q", got)
	}
}

func newRegistryV2RootTestServer(t *testing.T) *Server {
	t.Helper()

	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	s := &Server{
		router:                gin.New(),
		registryJWTPrivateKey: privateKey,
		registryJWTPublicKey:  publicKey,
		registryService:       "registry.test",
	}
	s.addRegistryRoutes()
	return s
}
