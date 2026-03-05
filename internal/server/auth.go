package server

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var errUnauthorized = errors.New("unauthorized")

type user struct {
	id        uuid.UUID
	sub       string
	tenantID  uuid.UUID
	onboarded bool
}

type workosJWTClaims struct {
	jwt.RegisteredClaims
	SID   string `json:"sid"`
	OrgID string `json:"org_id"`
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			slog.Debug("Auth", slog.String("Bearer", "Missing header"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - no header"})
			return
		}

		const prefix = "Bearer "
		if len(authHeader) < len(prefix) || !strings.EqualFold(authHeader[:len(prefix)], prefix) {
			slog.Debug("Auth", slog.String("Bearer", "Format"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - bearer format"})
			return
		}
		tokenString := strings.TrimSpace(authHeader[len(prefix):])

		var claims workosJWTClaims
		_, err := jwt.ParseWithClaims(tokenString, &claims, s.jwks.Keyfunc,
			jwt.WithIssuedAt(),
			jwt.WithExpirationRequired(),
			jwt.WithIssuer(fmt.Sprintf("https://api.workos.com/user_management/%s", s.workosClientID)),
		)
		if err != nil {
			slog.Debug("Auth", slog.String("Bearer", "Claims"), slog.String("err", err.Error()))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - jwt"})
			return
		}

		sub := strings.TrimSpace(claims.Subject)
		if sub == "" {
			slog.Debug("Auth", slog.String("Bearer", "Missing sub"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - sub"})
			return
		}

		org := strings.TrimSpace(claims.OrgID)
		dbUser, err := s.db.GetOrCreateUser(c.Request.Context(), sub, org)
		if err != nil {
			logError(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not resolve user"})
			return
		}

		u := user{
			id:        dbUser.ID,
			sub:       dbUser.Sub,
			tenantID:  dbUser.TenantID,
			onboarded: dbUser.Onboarded,
		}
		c.Set("user", u)
		c.Next()
	}
}

func (s *Server) getUser(c *gin.Context) (user, error) {
	userObj, exists := c.Get("user")
	if !exists {
		return user{}, errUnauthorized
	}
	u, ok := userObj.(user)
	if !ok {
		return user{}, errUnauthorized
	}
	return u, nil
}
