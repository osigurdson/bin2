package server

import (
	"context"
	"fmt"
	"os"

	"bin2.io/internal/db"
	"github.com/clerk/clerk-sdk-go/v2"
	"github.com/gin-gonic/gin"
)

type Server struct {
	ctx    context.Context
	router *gin.Engine
	db     *db.DB
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

	s := &Server{
		ctx:    context.Background(),
		router: gin.Default(),
		db:     conn,
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
