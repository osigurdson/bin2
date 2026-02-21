package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
)

var registryNameRe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

const maxRegistryNameLen = 64

type addRegistryRequest struct {
	Name string `json:"name"`
}

type registryResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type listRegistriesResponse struct {
	Registries []registryResponse `json:"registries"`
}

func validRegistryName(name string) bool {
	if len(name) == 0 || len(name) > maxRegistryNameLen {
		return false
	}
	return registryNameRe.MatchString(name)
}

func (s *Server) listRegistriesHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	registries, err := s.db.ListRegistriesByUser(c.Request.Context(), u.id)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list registries"})
		return
	}

	resp := listRegistriesResponse{
		Registries: make([]registryResponse, 0, len(registries)),
	}
	for _, registry := range registries {
		resp.Registries = append(resp.Registries, registryResponse{
			ID:   registry.ID.String(),
			Name: registry.Name,
		})
	}
	c.JSON(http.StatusOK, resp)
}

func (s *Server) addRegistryHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req addRegistryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !validRegistryName(req.Name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registry name"})
		return
	}

	registry, err := s.db.AddRegistry(c.Request.Context(), db.AddRegistryArgs{
		UserID: u.id,
		Name:   req.Name,
	})
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "registry name already exists"})
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create registry"})
		return
	}

	c.JSON(http.StatusCreated, registryResponse{
		ID:   registry.ID.String(),
		Name: registry.Name,
	})
}
