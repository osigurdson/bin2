package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegistryJWKSRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	s := &Server{
		router:               gin.New(),
		registryJWTPublicKey: publicKey,
	}
	s.addWellKnownRoutes()

	req := httptest.NewRequest(
		http.MethodGet,
		"/.well-known/jwks.json",
		nil,
	)
	res := httptest.NewRecorder()
	s.router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}
	if got := res.Header().Get("Cache-Control"); got == "" {
		t.Fatalf("missing Cache-Control header")
	}

	var body jwksResponse
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.Keys) != 1 {
		t.Fatalf("keys len = %d, want 1", len(body.Keys))
	}
	key := body.Keys[0]
	if key.Kty != "OKP" {
		t.Fatalf("kty = %q, want OKP", key.Kty)
	}
	if key.Alg != "EdDSA" {
		t.Fatalf("alg = %q, want EdDSA", key.Alg)
	}
	if key.Use != "sig" {
		t.Fatalf("use = %q, want sig", key.Use)
	}
	if key.Crv != "Ed25519" {
		t.Fatalf("crv = %q, want Ed25519", key.Crv)
	}
	if key.X == "" || key.Kid == "" {
		t.Fatalf("unexpected empty jwk fields: %#v", key)
	}
}
