package server

import (
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var keyNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{2,8}$`)

type createAPIKeyRequest struct {
	KeyName string              `json:"keyName"`
	Scopes  []createAPIKeyScope `json:"scopes"`
}

type createAPIKeyScope struct {
	RegistryID string  `json:"registryId"`
	Repository *string `json:"repository,omitempty"`
	Permission string  `json:"permission"`
}

type apiKeyResponse struct {
	ID         uuid.UUID             `json:"id"`
	KeyName    string                `json:"keyName"`
	Prefix     string                `json:"prefix"`
	SecretKey  string                `json:"secretKey"`
	CreatedAt  time.Time             `json:"createdAt"`
	LastUsedAt *time.Time            `json:"lastUsedAt,omitempty"`
	Scopes     []apiKeyScopeResponse `json:"scopes"`
}

type apiKeyScopeResponse struct {
	RegistryID uuid.UUID `json:"registryId"`
	Repository *string   `json:"repository,omitempty"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"createdAt"`
}

type listAPIKeysResponse struct {
	Keys []apiKeyResponse `json:"keys"`
}

type apiKeyRequestError struct {
	message string
}

func (e apiKeyRequestError) Error() string {
	return e.message
}

func (s *Server) buildAPIKeyResponse(key db.APIKey, secretKey string) apiKeyResponse {
	return apiKeyResponse{
		ID:         key.ID,
		KeyName:    key.KeyName,
		Prefix:     key.Prefix,
		SecretKey:  secretKey,
		CreatedAt:  key.CreatedAt,
		LastUsedAt: key.LastUsedAt,
		Scopes:     apiKeyScopesResponse(key.Scopes),
	}
}

func (s *Server) addAPIKeyHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req createAPIKeyRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read request body"})
		return
	}

	if !keyNameRe.MatchString(req.KeyName) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Must be 2-8 chars of letters, numbers, '.', '_' or '-'"})
		return
	}

	scopes, err := s.resolveCreateAPIKeyScopes(c, u, req.Scopes)
	if err != nil {
		if errors.Is(err, errUnauthorized) {
			c.JSON(http.StatusForbidden, gin.H{"error": "not allowed to create key for that registry"})
			return
		}
		var reqErr apiKeyRequestError
		if errors.As(err, &reqErr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": reqErr.Error()})
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	fullKey, prefix, err := generateAPIKey()
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}
	encrypted, err := encryptAPIKey(fullKey, s.apiKeyEncryptionKey)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	apiKeyRec, err := s.db.AddAPIKey(c.Request.Context(), db.AddAPIKeyArgs{
		UserID:          u.id,
		KeyName:         req.KeyName,
		SecretEncrypted: encrypted,
		Prefix:          prefix,
		Scopes:          scopes,
	})
	if err != nil {
		if errors.Is(err, db.ErrScopeConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "duplicate scope"})
			return
		}
		if errors.Is(err, db.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "key name already exists"})
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, s.buildAPIKeyResponse(apiKeyRec, fullKey))
}

func (s *Server) listAPIKeysHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	apiKeyRecs, err := s.db.ListAPIKeysByUser(c.Request.Context(), u.id)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	keys := make([]apiKeyResponse, 0, len(apiKeyRecs))
	for _, rec := range apiKeyRecs {
		fullKey, err := decryptAPIKey(rec.SecretEncrypted, s.apiKeyEncryptionKey)
		if err != nil {
			logError(err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		keys = append(keys, s.buildAPIKeyResponse(rec, fullKey))
	}

	c.JSON(http.StatusOK, listAPIKeysResponse{Keys: keys})
}

func (s *Server) removeAPIKeyHandler(c *gin.Context) {
	u, err := s.getUser(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Key ID malformed"})
		return
	}

	err = s.db.RemoveAPIKey(c.Request.Context(), u.id, id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func apiKeyScopesResponse(scopes []db.APIKeyScope) []apiKeyScopeResponse {
	out := make([]apiKeyScopeResponse, 0, len(scopes))
	for _, scope := range scopes {
		out = append(out, apiKeyScopeResponse{
			RegistryID: scope.RegistryID,
			Repository: scope.Repository,
			Permission: string(scope.Permission),
			CreatedAt:  scope.CreatedAt,
		})
	}
	return out
}

func (s *Server) resolveCreateAPIKeyScopes(c *gin.Context, u user, rawScopes []createAPIKeyScope) ([]db.AddAPIKeyScopeInput, error) {
	if len(rawScopes) == 0 {
		return nil, apiKeyRequestError{message: "at least one scope is required"}
	}

	type normalizedScope struct {
		registryID uuid.UUID
		repository string
	}

	out := make([]db.AddAPIKeyScopeInput, 0, len(rawScopes))
	seen := map[normalizedScope]struct{}{}

	for _, rawScope := range rawScopes {
		registryID, err := uuid.Parse(strings.TrimSpace(rawScope.RegistryID))
		if err != nil {
			return nil, apiKeyRequestError{message: "registryId is malformed"}
		}

		registryRec, err := s.db.GetRegistryByID(c.Request.Context(), registryID)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				return nil, apiKeyRequestError{message: "registry does not exist"}
			}
			return nil, err
		}

		isMember, err := s.db.IsOrgMember(c.Request.Context(), registryRec.OrgID, u.id)
		if err != nil {
			return nil, err
		}
		if !isMember {
			return nil, errUnauthorized
		}

		permission, err := parseAPIKeyPermission(rawScope.Permission)
		if err != nil {
			return nil, err
		}

		var repositoryID *uuid.UUID
		normalizedRepository := ""
		if rawScope.Repository != nil {
			repo := strings.TrimSpace(*rawScope.Repository)
			if repo != "" {
				if !validRepoName(repo) {
					return nil, apiKeyRequestError{message: "repository name is invalid"}
				}
				if registryNamespace(repo) != registryRec.Name {
					return nil, apiKeyRequestError{message: "repository must belong to the selected registry"}
				}
				leaf := repoLeaf(repo)
				repositoryRec, err := s.db.EnsureRegistryRepository(c.Request.Context(), registryID, leaf)
				if err != nil {
					return nil, err
				}
				repositoryID = &repositoryRec.ID
				normalizedRepository = repositoryRec.ID.String()
			}
		}

		scopeKey := normalizedScope{registryID: registryID, repository: normalizedRepository}
		if _, ok := seen[scopeKey]; ok {
			return nil, apiKeyRequestError{message: "duplicate scope target"}
		}
		seen[scopeKey] = struct{}{}

		out = append(out, db.AddAPIKeyScopeInput{
			RegistryID:   registryID,
			RepositoryID: repositoryID,
			Permission:   permission,
		})
	}

	slices.SortFunc(out, func(a, b db.AddAPIKeyScopeInput) int {
		if a.RegistryID != b.RegistryID {
			return strings.Compare(a.RegistryID.String(), b.RegistryID.String())
		}
		aRepo := uuid.Nil.String()
		if a.RepositoryID != nil {
			aRepo = a.RepositoryID.String()
		}
		bRepo := uuid.Nil.String()
		if b.RepositoryID != nil {
			bRepo = b.RepositoryID.String()
		}
		return strings.Compare(aRepo, bRepo)
	})

	return out, nil
}

func parseAPIKeyPermission(raw string) (db.APIKeyPermission, error) {
	switch strings.TrimSpace(raw) {
	case string(db.APIKeyPermissionRead):
		return db.APIKeyPermissionRead, nil
	case string(db.APIKeyPermissionWrite):
		return db.APIKeyPermissionWrite, nil
	case string(db.APIKeyPermissionAdmin):
		return db.APIKeyPermissionAdmin, nil
	default:
		return "", apiKeyRequestError{message: "permission must be read, write, or admin"}
	}
}
