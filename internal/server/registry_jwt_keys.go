package server

import (
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"strings"

	"encoding/pem"
)

func loadRegistryJWTKeys() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	privatePEM := strings.TrimSpace(getenvDefault(
		"REGISTRY_JWT_PRIVATE_KEY_PEM",
		"",
	))
	if privatePEM == "" {
		return nil, nil, fmt.Errorf("REGISTRY_JWT_PRIVATE_KEY_PEM is required")
	}

	privateKey, err := parseEd25519PrivateKey(privatePEM)
	if err != nil {
		return nil, nil, err
	}

	publicPEM := strings.TrimSpace(getenvDefault(
		"REGISTRY_JWT_PUBLIC_KEY_PEM",
		"",
	))
	if publicPEM == "" {
		derived := privateKey.Public().(ed25519.PublicKey)
		return privateKey, derived, nil
	}

	publicKey, err := parseEd25519PublicKey(publicPEM)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, publicKey, nil
}

func parseEd25519PrivateKey(raw string) (ed25519.PrivateKey, error) {
	decoded := decodeRegistryPEM(raw)
	block, _ := pem.Decode([]byte(decoded))
	if block == nil {
		return nil, fmt.Errorf("could not decode private key PEM")
	}

	if block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("private key must be PKCS8 PRIVATE KEY")
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse PKCS8 private key: %w", err)
	}
	privateKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("private key must be Ed25519")
	}
	return privateKey, nil
}

func parseEd25519PublicKey(raw string) (ed25519.PublicKey, error) {
	decoded := decodeRegistryPEM(raw)
	block, _ := pem.Decode([]byte(decoded))
	if block == nil {
		return nil, fmt.Errorf("could not decode public key PEM")
	}

	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("public key must be PKIX PUBLIC KEY")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("could not parse PKIX public key: %w", err)
	}
	publicKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("public key must be Ed25519")
	}
	return publicKey, nil
}

func decodeRegistryPEM(raw string) string {
	if strings.Contains(raw, "\\n") {
		return strings.ReplaceAll(raw, "\\n", "\n")
	}
	return raw
}
