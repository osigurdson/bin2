package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// listUsageEventsHandler handles GET /api/v1/usage/events.
// Accepts either a WorkOS JWT (management API) or a registry bearer token
// (for testing/CF worker use). Returns events scoped to the authenticated tenant.
func (s *Server) listUsageEventsHandler(c *gin.Context) {
	tenantID, _, err := s.resolveUsageAuth(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	metric := strings.TrimSpace(c.Query("metric"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	var after time.Time
	if afterStr := strings.TrimSpace(c.Query("after")); afterStr != "" {
		after, _ = time.Parse(time.RFC3339, afterStr)
	}

	events, err := s.db.ListUsageEventsByTenant(c.Request.Context(), tenantID, metric, limit, after)
	if err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list usage events"})
		return
	}

	type eventResponse struct {
		ID         string  `json:"id"`
		CreatedAt  string  `json:"createdAt"`
		RegistryID *string `json:"registryId,omitempty"`
		RepoID     *string `json:"repoId,omitempty"`
		Digest     string  `json:"digest,omitempty"`
		Metric     string  `json:"metric"`
		Value      int64   `json:"value"`
	}

	out := make([]eventResponse, 0, len(events))
	for _, e := range events {
		resp := eventResponse{
			ID:        e.ID.String(),
			CreatedAt: e.CreatedAt.UTC().Format(time.RFC3339),
			Digest:    e.Digest,
			Metric:    e.Metric,
			Value:     e.Value,
		}
		if e.RegistryID != nil {
			s := e.RegistryID.String()
			resp.RegistryID = &s
		}
		if e.RepoID != nil {
			s := e.RepoID.String()
			resp.RepoID = &s
		}
		out = append(out, resp)
	}
	c.JSON(http.StatusOK, out)
}

// ingestUsageEventsHandler handles POST /api/v1/usage/events.
// Used by the CF worker to push pull-op-count events. Requires a valid
// registry bearer token; events are scoped to that token's registry.
func (s *Server) ingestUsageEventsHandler(c *gin.Context) {
	tenantID, registryID, err := s.resolveUsageAuth(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	type ingestRequest struct {
		ID     string `json:"id"`
		RepoID string `json:"repoId"`
		Digest string `json:"digest"`
		Metric string `json:"metric"`
		Value  int64  `json:"value"`
	}

	var body []ingestRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	events := make([]db.UsageEvent, 0, len(body))
	for _, req := range body {
		id, err := uuid.Parse(req.ID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event id: " + req.ID})
			return
		}
		if req.Metric != db.MetricPullOpCount && req.Metric != db.MetricPushOpCount && req.Metric != db.MetricStorageBytes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid metric: " + req.Metric})
			return
		}
		digest, err := normalizeUsageEventDigest(req.Digest)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid digest: " + req.Digest})
			return
		}
		event := db.UsageEvent{
			ID:       id,
			TenantID: tenantID,
			Digest:   digest,
			Metric:   req.Metric,
			Value:    req.Value,
		}
		if registryID != uuid.Nil {
			event.RegistryID = &registryID
		}
		if req.RepoID != "" {
			if repoID, err := uuid.Parse(req.RepoID); err == nil {
				event.RepoID = &repoID
			}
		}
		events = append(events, event)
	}

	if err := s.db.InsertUsageEvents(c.Request.Context(), events); err != nil {
		logError(err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store events"})
		return
	}

	c.Status(http.StatusNoContent)
}

// resolveUsageAuth tries WorkOS JWT auth first, then falls back to registry
// bearer token auth. Returns (tenantID, registryID, error).
// registryID is uuid.Nil when authenticated via WorkOS JWT (tenant-scoped).
func (s *Server) resolveUsageAuth(c *gin.Context) (tenantID, registryID uuid.UUID, err error) {
	// Try WorkOS JWT (management API auth).
	if u, userErr := s.getUser(c); userErr == nil {
		return u.tenantID, uuid.Nil, nil
	}

	// Fall back to registry bearer token.
	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(authHeader) < 8 || !strings.EqualFold(authHeader[:7], "bearer ") {
		return uuid.Nil, uuid.Nil, errUnauthorized
	}
	tokenString := strings.TrimSpace(authHeader[7:])
	service := s.registryServiceForRequest(c)
	claims, tokenErr := s.verifyRegistryToken(tokenString, service)
	if tokenErr != nil {
		return uuid.Nil, uuid.Nil, errUnauthorized
	}

	namespace := strings.TrimSpace(claims.Subject)
	reg, regErr := s.db.GetRegistryByName(c.Request.Context(), namespace)
	if regErr != nil {
		return uuid.Nil, uuid.Nil, errUnauthorized
	}
	return reg.TenantID, reg.ID, nil
}
