package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistryV2RequiresAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	s := &Server{
		router:          gin.New(),
		registryStorage: newLocalRegistryStorage(t.TempDir()),
	}
	s.addRegistryRoutes()

	req := httptest.NewRequest(http.MethodGet, "/v2/", nil)
	res := httptest.NewRecorder()
	s.router.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusUnauthorized)
	}
	if got := res.Header().Get("Docker-Distribution-API-Version"); got != "registry/2.0" {
		t.Fatalf("Docker-Distribution-API-Version = %q", got)
	}
	if got := res.Header().Get("WWW-Authenticate"); !strings.HasPrefix(got, `Bearer realm=`) {
		t.Fatalf("WWW-Authenticate = %q", got)
	}
}
