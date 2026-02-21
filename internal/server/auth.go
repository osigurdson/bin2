package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"bin2.io/internal/db"
	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
	clerkuser "github.com/clerk/clerk-sdk-go/v2/user"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var errUnauthorized = errors.New("unauthorized")

type user struct {
	id    uuid.UUID
	sub   string
	email string
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		clerkMiddleware := clerkhttp.WithHeaderAuthorization()
		authHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := clerk.SessionClaimsFromContext(r.Context())
			if !ok || strings.TrimSpace(claims.Subject) == "" {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
				return
			}

			u, err := s.getOrCreateUser(c.Request.Context(), claims.Subject)
			if err != nil {
				logError(err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "could not resolve user"})
				return
			}

			c.Set("user", u)
			c.Next()
		})

		clerkMiddleware(authHandler).ServeHTTP(c.Writer, c.Request)
	}
}

func (s *Server) getOrCreateUser(ctx context.Context, sub string) (user, error) {
	dbUser, err := s.db.GetUserBySub(ctx, sub)
	if err == nil {
		return user{
			id:    dbUser.ID,
			sub:   dbUser.Sub,
			email: dbUser.Email,
		}, nil
	}
	if !errors.Is(err, db.ErrNotFound) {
		return user{}, err
	}

	email, err := s.getClerkUserEmail(ctx, sub)
	if err != nil {
		return user{}, err
	}

	dbUser, err = s.db.EnsureUser(ctx, sub, email)
	if err != nil {
		if errors.Is(err, db.ErrConflict) {
			// Recover if this email already exists under another sub.
			byEmail, lookupErr := s.db.GetUserByEmail(ctx, email)
			if lookupErr == nil {
				if byEmail.Sub != sub {
					updated, updateErr := s.db.UpdateUserSub(ctx, byEmail.ID, sub)
					if updateErr == nil {
						byEmail = updated
					}
				}
				return user{
					id:    byEmail.ID,
					sub:   byEmail.Sub,
					email: byEmail.Email,
				}, nil
			}
		}
		return user{}, err
	}

	return user{
		id:    dbUser.ID,
		sub:   dbUser.Sub,
		email: dbUser.Email,
	}, nil
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

func (s *Server) getClerkUserEmail(ctx context.Context, sub string) (string, error) {
	cu, err := clerkuser.Get(ctx, sub)
	if err != nil {
		return "", err
	}

	if cu.PrimaryEmailAddressID == nil {
		return "", errors.New("primary email not available from clerk")
	}

	for _, address := range cu.EmailAddresses {
		if address.ID == *cu.PrimaryEmailAddressID {
			return address.EmailAddress, nil
		}
	}

	return "", errors.New("primary email address not found")
}
