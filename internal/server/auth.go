package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/workos/workos-go/v4/pkg/usermanagement"
)

var errUnauthorized = errors.New("unauthorized")

type user struct {
	id    uuid.UUID
	sub   string
	email string
	orgID uuid.UUID
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
			jwt.WithIssuer("https://api.workos.com"),
			jwt.WithAudience(s.workosClientID),
		)
		if err != nil {
			slog.Debug("Auth", slog.String("Bearer", "Claims"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - jwt"})
			return
		}

		sub := strings.TrimSpace(claims.Subject)
		if sub == "" {
			slog.Debug("Auth", slog.String("Bearer", "Missing sub"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized - sub"})
			return
		}

		u, err := s.getOrCreateUser(c.Request.Context(), sub)
		if err != nil {
			logError(err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not resolve user"})
			return
		}

		c.Set("user", u)
		c.Next()
	}
}

func (s *Server) getOrCreateUser(ctx context.Context, sub string) (user, error) {
	dbUser, err := s.db.GetUserBySub(ctx, sub)
	if err == nil {
		orgID, err := s.ensurePersonalOrg(ctx, dbUser.ID, dbUser.Sub)
		if err != nil {
			return user{}, err
		}
		return user{
			id:    dbUser.ID,
			sub:   dbUser.Sub,
			email: dbUser.Email,
			orgID: orgID,
		}, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return user{}, err
	}

	email, err := s.getWorkOSUserEmail(ctx, sub)
	if err != nil {
		return user{}, err
	}

	dbUser, err = s.db.EnsureUser(ctx, sub, email)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			byEmail, lookupErr := s.db.GetUserByEmail(ctx, email)
			if lookupErr == nil {
				if byEmail.Sub != sub {
					updated, updateErr := s.db.UpdateUserSub(ctx, byEmail.ID, sub)
					if updateErr == nil {
						byEmail = updated
					}
				}
				orgID, orgErr := s.ensurePersonalOrg(ctx, byEmail.ID, byEmail.Sub)
				if orgErr != nil {
					return user{}, orgErr
				}
				return user{
					id:    byEmail.ID,
					sub:   byEmail.Sub,
					email: byEmail.Email,
					orgID: orgID,
				}, nil
			}
		}
		return user{}, err
	}

	orgID, err := s.ensurePersonalOrg(ctx, dbUser.ID, dbUser.Sub)
	if err != nil {
		return user{}, err
	}

	return user{
		id:    dbUser.ID,
		sub:   dbUser.Sub,
		email: dbUser.Email,
		orgID: orgID,
	}, nil
}

func (s *Server) ensurePersonalOrg(ctx context.Context, userID uuid.UUID, sub string) (uuid.UUID, error) {
	org, err := s.db.GetPersonalOrgByUser(ctx, userID)
	if err == nil {
		return org.ID, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return uuid.UUID{}, err
	}

	orgID := uuid.New()
	slug := fmt.Sprintf("personal-%s", sub)
	org, err = s.db.CreateOrganization(ctx, orgID, slug, "Personal", nil)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			// Another request may have created it concurrently.
			org, err = s.db.GetPersonalOrgByUser(ctx, userID)
			if err != nil {
				return uuid.UUID{}, err
			}
			return org.ID, nil
		}
		return uuid.UUID{}, err
	}

	if err := s.db.AddOrgMember(ctx, org.ID, userID, "owner"); err != nil {
		return uuid.UUID{}, err
	}

	return org.ID, nil
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

func (s *Server) getWorkOSUserEmail(ctx context.Context, sub string) (string, error) {
	wu, err := usermanagement.GetUser(ctx, usermanagement.GetUserOpts{User: sub})
	if err != nil {
		return "", err
	}
	return wu.Email, nil
}
