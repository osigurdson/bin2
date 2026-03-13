package server

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"bin2.io/internal/db"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/workos/workos-go/v4/pkg/usermanagement"
)

type probeCache struct {
	mu     sync.Mutex
	recent map[string]time.Time
}

func (p *probeCache) shouldUpdate(digest string) bool {
	const debounce = 30 * time.Second
	now := time.Now()
	p.mu.Lock()
	defer p.mu.Unlock()
	if t, ok := p.recent[digest]; ok && now.Sub(t) < debounce {
		return false
	}
	p.recent[digest] = now
	return true
}

type Server struct {
	ctx                   context.Context
	router                *gin.Engine
	db                    *db.DB
	registryStorage       *r2RegistryStorage
	registryJWTPrivateKey ed25519.PrivateKey
	registryJWTPublicKey  ed25519.PublicKey
	registryService       string
	jwks                  keyfunc.Keyfunc
	workosClientID        string
	apiKeyEncryptionKey   [32]byte
	probeCache            *probeCache
	usageIngestSecret     string
}

func New() (*Server, error) {
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

	workosAPIKey := os.Getenv("WORKOS_API_KEY")
	if workosAPIKey == "" {
		return nil, fmt.Errorf("WORKOS_API_KEY is not defined")
	}

	workosClientID := os.Getenv("WORKOS_CLIENT_ID")
	if workosClientID == "" {
		return nil, fmt.Errorf("WORKOS_CLIENT_ID is not defined")
	}

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
		return nil, fmt.Errorf("could not initialize registry storage: %w", err)
	}

	registryJWTPrivateKey, registryJWTPublicKey, err := loadRegistryJWTKeys()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("could not load registry jwt keys: %w", err)
	}

	usageIngestSecret := strings.TrimSpace(os.Getenv("USAGE_INGEST_SECRET"))
	if usageIngestSecret == "" {
		conn.Close()
		return nil, fmt.Errorf("USAGE_INGEST_SECRET is not defined")
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
		probeCache:            &probeCache{recent: make(map[string]time.Time)},
		usageIngestSecret:     usageIngestSecret,
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
