package server

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

type addRegistryResponse struct {
	ID     string         `json:"id"`
	Name   string         `json:"name"`
	APIKey apiKeyResponse `json:"apiKey"`
}

type listRegistriesResponse struct {
	Registries []registryResponse `json:"registries"`
}

type repositoryResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	LastPush string  `json:"lastPush"`
	LastTag  *string `json:"lastTag"`
}

type listRepositoriesResponse struct {
	Repositories []repositoryResponse `json:"repositories"`
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

	registries, err := s.db.ListRegistriesByOrg(c.Request.Context(), u.orgID)
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

func (s *Server) getRegistryByIDHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idParam := strings.TrimSpace(c.Param("id"))
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registry id"})
		return
	}

	registry, err := s.db.GetRegistryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "registry not found"})
			return
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get registry"})
		return
	}

	if registry.OrgID != u.orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "registry not found"})
		return
	}

	c.JSON(http.StatusOK, registryResponse{
		ID:   registry.ID.String(),
		Name: registry.Name,
	})
}

func (s *Server) listRepositoriesHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	idParam := strings.TrimSpace(c.Query("registryId"))
	if idParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "registryId is required"})
		return
	}
	id, err := uuid.Parse(idParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid registryId"})
		return
	}

	registry, err := s.db.GetRegistryByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "registry not found"})
			return
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not get registry"})
		return
	}

	if registry.OrgID != u.orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "registry not found"})
		return
	}

	repositories, err := s.db.ListRepositoriesByRegistryID(c.Request.Context(), registry.ID)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not list repositories"})
		return
	}

	resp := listRepositoriesResponse{
		Repositories: make([]repositoryResponse, 0, len(repositories)),
	}
	for _, repository := range repositories {
		resp.Repositories = append(resp.Repositories, repositoryResponse{
			ID:       repository.ID.String(),
			Name:     repository.Name,
			LastPush: repository.LastPushedAt.UTC().Format(time.RFC3339),
			LastTag:  repository.LastTag,
		})
	}

	c.JSON(http.StatusOK, resp)
}

func (s *Server) getRegistryExistsHandler(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if !validRegistryName(name) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad registry name"})
		return
	}

	_, err := s.db.GetRegistryByName(c.Request.Context(), name)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusOK, false)
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server error"})
		return
	}

	c.JSON(http.StatusOK, true)
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

	fullKey, prefix, err := generateAPIKey()
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create registry"})
		return
	}
	encrypted, err := encryptAPIKey(fullKey, s.apiKeyEncryptionKey)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create registry"})
		return
	}

	result, err := s.db.AddRegistryWithKey(c.Request.Context(), db.AddRegistryWithKeyArgs{
		OrgID:           u.orgID,
		Name:            req.Name,
		UserID:          u.id,
		KeyName:         "default",
		SecretEncrypted: encrypted,
		Prefix:          prefix,
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

	c.JSON(http.StatusCreated, addRegistryResponse{
		ID:     result.Registry.ID.String(),
		Name:   result.Registry.Name,
		APIKey: s.buildAPIKeyResponse(result.APIKey, fullKey),
	})
}
