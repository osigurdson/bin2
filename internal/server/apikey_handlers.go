package server

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var keyNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{2,8}$`)

type createAPIKeyRequest struct {
	KeyName string `json:"keyName"`
}

type apiKeyResponse struct {
	ID         uuid.UUID  `json:"id"`
	KeyName    string     `json:"keyName"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`
}

type createAPIKeyResponse struct {
	APIKey    apiKeyResponse `json:"apiKey"`
	SecretKey string         `json:"secretKey"`
}

type listAPIKeysResponse struct {
	Keys []apiKeyResponse `json:"keys"`
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

	cred, hash, err := generateAPIKeyCredential(u.id)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	apiKeyRec := db.APIKey{
		ID:            uuid.New(),
		UserID:        u.id,
		KeyName:       req.KeyName,
		SecretKeyHash: hash,
		Prefix:        cred.UserID.String(),
	}

	apiKeyRec, err = s.db.AddAPIKey(c.Request.Context(), apiKeyRec)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "key name already exists"})
			return
		}
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	resp := createAPIKeyResponse{
		APIKey: apiKeyResponse{
			ID:         apiKeyRec.ID,
			KeyName:    apiKeyRec.KeyName,
			Prefix:     apiKeyRec.Prefix,
			CreatedAt:  apiKeyRec.CreatedAt,
			LastUsedAt: apiKeyRec.LastUsedAt,
		},
		SecretKey: ConvertAPIKeyCredentialToString(cred),
	}

	c.JSON(http.StatusCreated, resp)
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
	for _, apiKeyRec := range apiKeyRecs {
		keys = append(keys, apiKeyResponse{
			ID:         apiKeyRec.ID,
			KeyName:    apiKeyRec.KeyName,
			Prefix:     apiKeyRec.Prefix,
			CreatedAt:  apiKeyRec.CreatedAt,
			LastUsedAt: apiKeyRec.LastUsedAt,
		})
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
