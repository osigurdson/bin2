package server

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"net/http"

	"github.com/gin-gonic/gin"
)

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Use string `json:"use,omitempty"`
	Kid string `json:"kid,omitempty"`
	Alg string `json:"alg,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
}

func (s *Server) addWellKnownRoutes() {
	s.router.GET("/.well-known/jwks.json", s.registryJWKSHandler)
}

func (s *Server) registryJWKSHandler(c *gin.Context) {
	publicKey := s.registryJWTPublicKey
	if publicKey == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "registry public key is not configured",
		})
		return
	}

	if len(publicKey) != ed25519.PublicKeySize {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "registry public key is invalid",
		})
		return
	}

	encodedX := base64.RawURLEncoding.EncodeToString(publicKey)

	keyID, err := registryJWKKeyID(publicKey)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "could not encode registry public key",
		})
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.JSON(http.StatusOK, jwksResponse{
		Keys: []jwk{{
			Kty: "OKP",
			Use: "sig",
			Kid: keyID,
			Alg: "EdDSA",
			Crv: "Ed25519",
			X:   encodedX,
		}},
	})
}

func registryJWKKeyID(publicKey any) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}
