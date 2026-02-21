package server

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const registryTokenTTL = 30 * time.Minute

type registryTokenAccess struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

type registryTokenClaims struct {
	Access []registryTokenAccess `json:"access,omitempty"`
	jwt.RegisteredClaims
}

type registryTokenResponse struct {
	Token       string `json:"token"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	IssuedAt    string `json:"issued_at"`
}

func (s *Server) registryTokenHandler(c *gin.Context) {
	if c.Request.Method != http.MethodGet {
		writeOCIError(c, http.StatusMethodNotAllowed, "UNSUPPORTED", "method not allowed")
		return
	}

	auth, err := s.authenticateRegistryBasic(c)
	if err != nil {
		if errors.Is(err, errUnauthorized) {
			writeOCIUnauthorized(c)
			return
		}
		logError(err)
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "internal server error")
		return
	}

	service := strings.TrimSpace(c.Query("service"))
	if service == "" {
		service = s.registryServiceForRequest(c)
	}

	requestedScopes := parseRequestedTokenScopes(c.QueryArray("scope"))
	grantedScopes := grantRegistryTokenScopes(auth.namespace, requestedScopes)

	token, expiresAt, issuedAt, err := s.issueRegistryToken(auth.namespace, service, grantedScopes)
	if err != nil {
		logError(err)
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to issue token")
		return
	}

	c.JSON(http.StatusOK, registryTokenResponse{
		Token:       token,
		AccessToken: token,
		ExpiresIn:   int64(expiresAt.Sub(issuedAt).Seconds()),
		IssuedAt:    issuedAt.UTC().Format(time.RFC3339),
	})
}

func (s *Server) issueRegistryToken(namespace, service string, access []registryTokenAccess) (string, time.Time, time.Time, error) {
	issuedAt := time.Now().UTC()
	expiresAt := issuedAt.Add(registryTokenTTL)

	claims := registryTokenClaims{
		Access: access,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    service,
			Subject:   namespace,
			Audience:  jwt.ClaimStrings{service},
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt.Add(-30 * time.Second)),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(s.registryJWTKey))
	if err != nil {
		return "", time.Time{}, time.Time{}, err
	}
	return signed, expiresAt, issuedAt, nil
}

func (s *Server) verifyRegistryToken(tokenString, service string) (*registryTokenClaims, error) {
	claims := &registryTokenClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (any, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(s.registryJWTKey), nil
		},
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("token invalid")
	}

	if service != "" && !slices.Contains(claims.Audience, service) {
		return nil, fmt.Errorf("token audience mismatch")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("token subject missing")
	}
	return claims, nil
}

func registryTokenAllows(access []registryTokenAccess, repository, action string) bool {
	if repository == "" || action == "" {
		return true
	}

	for _, granted := range access {
		if granted.Type != "repository" {
			continue
		}
		if granted.Name != repository {
			continue
		}
		for _, candidate := range granted.Actions {
			if candidate == action || candidate == "*" {
				return true
			}
		}
	}
	return false
}

func parseRequestedTokenScopes(rawScopes []string) []registryTokenAccess {
	scopes := make([]registryTokenAccess, 0, len(rawScopes))
	for _, raw := range rawScopes {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) != 3 {
			continue
		}
		actions := make([]string, 0)
		for _, action := range strings.Split(parts[2], ",") {
			action = strings.TrimSpace(action)
			if action == "" {
				continue
			}
			actions = append(actions, action)
		}
		scopes = append(scopes, registryTokenAccess{
			Type:    strings.TrimSpace(parts[0]),
			Name:    strings.TrimSpace(parts[1]),
			Actions: actions,
		})
	}
	return scopes
}

func grantRegistryTokenScopes(namespace string, requested []registryTokenAccess) []registryTokenAccess {
	type key struct {
		typeName string
		name     string
	}

	merged := map[key]map[string]struct{}{}
	for _, req := range requested {
		if req.Type != "repository" {
			continue
		}
		if registryNamespace(req.Name) != namespace {
			continue
		}
		k := key{typeName: req.Type, name: req.Name}
		if _, ok := merged[k]; !ok {
			merged[k] = map[string]struct{}{}
		}
		for _, action := range req.Actions {
			switch action {
			case "pull", "push", "*":
				merged[k][action] = struct{}{}
			}
		}
	}

	out := make([]registryTokenAccess, 0, len(merged))
	for k, actionSet := range merged {
		actions := make([]string, 0, len(actionSet))
		for action := range actionSet {
			actions = append(actions, action)
		}
		slices.Sort(actions)
		if len(actions) == 0 {
			continue
		}
		out = append(out, registryTokenAccess{
			Type:    k.typeName,
			Name:    k.name,
			Actions: actions,
		})
	}

	slices.SortFunc(out, func(a, b registryTokenAccess) int {
		if a.Type != b.Type {
			if a.Type < b.Type {
				return -1
			}
			return 1
		}
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})

	return out
}
