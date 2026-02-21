package server

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"bin2.io/internal/db"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-gonic/gin"
)

type Server struct {
	ctx             context.Context
	router          *gin.Engine
	db              *db.DB
	registryStorage registryStorage
	registryJWTKey  string
	registryService string
}

func New() (*Server, error) {
	clerkSecretKey := os.Getenv("CLERK_SECRET_KEY")
	if clerkSecretKey == "" {
		return nil, fmt.Errorf("CLERK_SECRET_KEY is not defined")
	}
	clerk.SetKey(clerkSecretKey)

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

	s := &Server{
		ctx:             context.Background(),
		router:          gin.Default(),
		db:              conn,
		registryStorage: rs,
		registryJWTKey:  strings.TrimSpace(getenvDefault("REGISTRY_JWT_KEY", "")),
		registryService: strings.TrimSpace(getenvDefault("REGISTRY_SERVICE", "")),
	}
	if s.registryJWTKey == "" {
		s.registryJWTKey = "bin2-dev-registry-jwt-key"
		slog.Warn("REGISTRY_JWT_KEY not set; using insecure development fallback key")
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
