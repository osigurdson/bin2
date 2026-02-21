package server

import (
	"context"

	"github.com/gin-gonic/gin"
)

type Server struct {
	ctx    context.Context
	router *gin.Engine
}

func New() (*Server, error) {
	s := &Server{
		ctx:    context.Background(),
		router: gin.Default(),
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

func (s *Server) Close() {}
