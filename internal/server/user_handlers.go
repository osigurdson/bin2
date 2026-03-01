package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type currentUserResponse struct {
	Onboarded bool `json:"onboarded"`
}

func (s *Server) getCurrentUserHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	c.JSON(http.StatusOK, currentUserResponse{
		Onboarded: u.onboarded,
	})
}
