package server

import (
	"errors"
	"fmt"
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

type registryScopeRequirement struct {
	repository string
	action     string
}

func (s *Server) apiVersionMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Docker-Distribution-API-Version", "registry/2.0")
		c.Next()
	}
}

func (s *Server) registryBearerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		relative := strings.TrimPrefix(c.Param("path"), "/")
		if isRegistryTokenPath(relative) {
			c.Next()
			return
		}

		reqScope, challengeScope := requiredRegistryScope(relative, c.Request.Method)
		realm := s.registryTokenRealm(c)
		service := s.registryServiceForRequest(c)

		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if len(authHeader) < len("Bearer ") || !strings.EqualFold(authHeader[:len("Bearer ")], "Bearer ") {
			writeOCIUnauthorizedBearer(c, realm, service, challengeScope)
			c.Abort()
			return
		}
		tokenString := strings.TrimSpace(authHeader[len("Bearer "):])
		if tokenString == "" {
			writeOCIUnauthorizedBearer(c, realm, service, challengeScope)
			c.Abort()
			return
		}

		claims, err := s.verifyRegistryToken(tokenString, service)
		if err != nil {
			writeOCIUnauthorizedBearer(c, realm, service, challengeScope)
			c.Abort()
			return
		}

		namespace := strings.TrimSpace(claims.Subject)
		if !validRegistryName(namespace) {
			writeOCIUnauthorizedBearer(c, realm, service, challengeScope)
			c.Abort()
			return
		}

		if reqScope.repository != "" {
			if !registryTokenAllows(claims.Access, reqScope.repository, reqScope.action) {
				setBearerAuthChallenge(c, realm, service, challengeScope)
				writeOCIError(c, http.StatusUnauthorized, "DENIED", "requested access to the resource is denied")
				c.Abort()
				return
			}
		}

		c.Set("registryAuth", registryAuthContext{
			namespace: namespace,
		})
		c.Next()
	}
}

func (s *Server) authenticateRegistryBasic(c *gin.Context) (registryAuthContext, error) {
	username, password, ok := c.Request.BasicAuth()
	if !ok {
		return registryAuthContext{}, errUnauthorized
	}

	namespace := strings.TrimSpace(username)
	if !validRegistryName(namespace) {
		return registryAuthContext{}, errUnauthorized
	}

	registryRec, err := s.db.GetRegistryByName(c.Request.Context(), namespace)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			return registryAuthContext{}, errUnauthorized
		}
		return registryAuthContext{}, err
	}

	cred, err := parseAPIKeyCredential(strings.TrimSpace(password))
	if err != nil || cred.UserID != registryRec.UserID {
		return registryAuthContext{}, errUnauthorized
	}

	keys, err := s.db.ListAPIKeysByUser(c.Request.Context(), cred.UserID)
	if err != nil {
		return registryAuthContext{}, err
	}

	matchedKey, err := matchAPIKeyCredential(cred, keys)
	if err != nil {
		return registryAuthContext{}, err
	}

	if _, err := s.db.UpdateAPIKeyLastUsedAt(c.Request.Context(), cred.UserID, matchedKey.ID); err != nil {
		logError(err)
	}

	return registryAuthContext{
		userID:    registryRec.UserID,
		namespace: namespace,
		apiKeyID:  matchedKey.ID,
	}, nil
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
		writeOCIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
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

func isRegistryTokenPath(relative string) bool {
	return strings.Trim(relative, "/") == "token"
}

func requiredRegistryScope(relativePath, method string) (registryScopeRequirement, string) {
	relative := strings.TrimPrefix(relativePath, "/")
	if relative == "" || relative == "/" {
		return registryScopeRequirement{}, ""
	}
	if isRegistryTokenPath(relative) {
		return registryScopeRequirement{}, ""
	}

	if method == http.MethodPost {
		if m := reStartUpload.FindStringSubmatch(relative); m != nil {
			req := registryScopeRequirement{repository: m[1], action: "push"}
			return req, formatRepositoryScope(req.repository, req.action)
		}
	}

	if method == http.MethodPatch || method == http.MethodPut {
		if m := reUploadChunk.FindStringSubmatch(relative); m != nil {
			req := registryScopeRequirement{repository: m[1], action: "push"}
			return req, formatRepositoryScope(req.repository, req.action)
		}
	}

	if method == http.MethodHead || method == http.MethodGet {
		if m := reBlobPath.FindStringSubmatch(relative); m != nil {
			req := registryScopeRequirement{repository: m[1], action: "pull"}
			return req, formatRepositoryScope(req.repository, req.action)
		}
	}

	if method == http.MethodPut || method == http.MethodHead || method == http.MethodGet {
		if m := reManifestRef.FindStringSubmatch(relative); m != nil {
			action := "pull"
			if method == http.MethodPut {
				action = "push"
			}
			req := registryScopeRequirement{repository: m[1], action: action}
			return req, formatRepositoryScope(req.repository, req.action)
		}
	}

	return registryScopeRequirement{}, ""
}

func formatRepositoryScope(repo, action string) string {
	return fmt.Sprintf("repository:%s:%s", repo, action)
}

func (s *Server) registryServiceForRequest(c *gin.Context) string {
	if s.registryService != "" {
		return s.registryService
	}
	host := strings.TrimSpace(c.Request.Host)
	if host != "" {
		return host
	}
	return "bin2-registry"
}

func (s *Server) registryTokenRealm(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" {
		host = s.registryServiceForRequest(c)
	}
	return fmt.Sprintf("%s://%s/v2/token", scheme, host)
}
