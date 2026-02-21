package server

import (
	"errors"
	"net/http"
	"strings"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type registryAuthContext struct {
	userID    uuid.UUID
	namespace string
	apiKeyID  uuid.UUID
}

func (s *Server) apiVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Next()
	}
}

func (s *Server) registryAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, password, ok := c.Request.BasicAuth()
		if !ok {
			writeOCIUnauthorized(c)
			c.Abort()
			return
		}

		namespace := strings.TrimSpace(username)
		if !validRegistryName(namespace) {
			writeOCIUnauthorized(c)
			c.Abort()
			return
		}

		registryRec, err := s.db.GetRegistryByName(c.Request.Context(), namespace)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeOCIUnauthorized(c)
				c.Abort()
				return
			}
			logError(err)
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "internal server error")
			c.Abort()
			return
		}

		cred, err := parseAPIKeyCredential(password)
		if err != nil || cred.UserID != registryRec.UserID {
			writeOCIUnauthorized(c)
			c.Abort()
			return
		}

		keys, err := s.db.ListAPIKeysByUser(c.Request.Context(), cred.UserID)
		if err != nil {
			logError(err)
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "internal server error")
			c.Abort()
			return
		}

		matchedKey, err := matchAPIKeyCredential(cred, keys)
		if err != nil {
			if errors.Is(err, errUnauthorized) {
				writeOCIUnauthorized(c)
				c.Abort()
				return
			}
			logError(err)
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "internal server error")
			c.Abort()
			return
		}

		if _, err := s.db.UpdateAPIKeyLastUsedAt(c.Request.Context(), cred.UserID, matchedKey.ID); err != nil {
			logError(err)
		}

		c.Set("registryAuth", registryAuthContext{
			userID:    registryRec.UserID,
			namespace: namespace,
			apiKeyID:  matchedKey.ID,
		})
		c.Next()
	}
}

func (s *Server) getRegistryAuth(c *gin.Context) (registryAuthContext, error) {
	obj, ok := c.Get("registryAuth")
	if !ok {
		return registryAuthContext{}, errUnauthorized
	}
	auth, ok := obj.(registryAuthContext)
	if !ok {
		return registryAuthContext{}, errUnauthorized
	}
	return auth, nil
}

func (s *Server) ensureRepoAuthorized(c *gin.Context, repo string) bool {
	auth, err := s.getRegistryAuth(c)
	if err != nil {
		writeOCIUnauthorized(c)
		return false
	}

	ns := registryNamespace(repo)
	if ns == "" {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return false
	}
	if ns != auth.namespace {
		writeOCIError(c, http.StatusForbidden, "DENIED", "access denied to this repository")
		return false
	}
	return true
}
