package server

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"bin2.io/internal/db"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/workos/workos-go/v4/pkg/usermanagement"
)

type Server struct {
	ctx                   context.Context
	router                *gin.Engine
	db                    *db.DB
	registryStorage       registryStorage
	registryJWTPrivateKey ed25519.PrivateKey
	registryJWTPublicKey  ed25519.PublicKey
	registryService       string
	jwks                  keyfunc.Keyfunc
	workosClientID        string
	apiKeyEncryptionKey   [32]byte
}

func New() (*Server, error) {
	workosAPIKey := os.Getenv("WORKOS_API_KEY")
	if workosAPIKey == "" {
		return nil, fmt.Errorf("WORKOS_API_KEY is not defined")
	}

	workosClientID := os.Getenv("WORKOS_CLIENT_ID")
	if workosClientID == "" {
		return nil, fmt.Errorf("WORKOS_CLIENT_ID is not defined")
	}

	apiKeyEncKeyHex := os.Getenv("API_KEY_ENCRYPTION_KEY")
	if apiKeyEncKeyHex == "" {
		return nil, fmt.Errorf("API_KEY_ENCRYPTION_KEY is not defined")
	}
	apiKeyEncKeyBytes, err := hex.DecodeString(apiKeyEncKeyHex)
	if err != nil || len(apiKeyEncKeyBytes) != 32 {
		return nil, fmt.Errorf("API_KEY_ENCRYPTION_KEY must be a 64-char hex string (32 bytes)")
	}
	var apiKeyEncryptionKey [32]byte
	copy(apiKeyEncryptionKey[:], apiKeyEncKeyBytes)

	usermanagement.SetAPIKey(workosAPIKey)

	jwksURL := fmt.Sprintf("https://api.workos.com/sso/jwks/%s", workosClientID)
	jwks, err := keyfunc.NewDefault([]string{jwksURL})
	if err != nil {
		return nil, fmt.Errorf("could not initialize JWKS: %w", err)
	}

	cfg, err := db.NewConfigFromEnv()
	if err != nil {
		return nil, fmt.Errorf("could not read postgres configuration: %w", err)
	}
	conn, err := db.New(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("could not connect to postgres: %w", err)
	}

	rs, err := newRegistryStorageFromEnv()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not initialize registry storage: %w", err)
	}
	if err := rs.Init(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not initialize registry storage backend: %w", err)
	}

	registryJWTPrivateKey, registryJWTPublicKey, err := loadRegistryJWTKeys()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not load registry jwt keys: %w", err)
	}

	s := &Server{
		ctx:                   context.Background(),
		router:                gin.Default(),
		db:                    conn,
		registryStorage:       rs,
		registryJWTPrivateKey: registryJWTPrivateKey,
		registryJWTPublicKey:  registryJWTPublicKey,
		registryService:       strings.TrimSpace(getenvDefault("REGISTRY_SERVICE", "")),
		jwks:                  jwks,
		workosClientID:        workosClientID,
		apiKeyEncryptionKey:   apiKeyEncryptionKey,
	}
	s.addRoutes()
	return s, nil
}

func (s *Server) Context() context.Context {
	return s.ctx
}

func (s *Server) Run(ctx context.Context, listen string) error {
	s.ctx = ctx
	return s.router.Run(listen)
}

func (s *Server) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
