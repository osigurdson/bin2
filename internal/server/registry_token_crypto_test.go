package server

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
)

func TestIssueAndVerifyRegistryTokenEdDSA(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	s := &Server{
		registryJWTPrivateKey: privateKey,
		registryJWTPublicKey:  publicKey,
	}

	token, _, _, err := s.issueRegistryToken("alpha", "localhost:5000", nil)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	claims, err := s.verifyRegistryToken(token, "localhost:5000")
	if err != nil {
		t.Fatalf("verify token: %v", err)
	}
	if claims.Subject != "alpha" {
		t.Fatalf("subject = %q, want alpha", claims.Subject)
	}

	if _, err := s.verifyRegistryToken(token, "localhost:9999"); err == nil {
		t.Fatalf("expected service mismatch error")
	}
}
