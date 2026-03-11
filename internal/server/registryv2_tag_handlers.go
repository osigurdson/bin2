package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) listTagsHandler(c *gin.Context, repo string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	auth, err := s.getRegistryAuth(c)
	if err != nil {
		writeOCIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	registryID, err := s.resolveRegistryIDForRepo(c.Request.Context(), auth, repo)
	if err != nil {
		if errors.Is(err, errUnauthorized) {
			writeOCIError(c, http.StatusForbidden, "DENIED", "access denied to this repository")
			return
		}
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to resolve registry")
		return
	}

	limit := 100
	if rawLimit := strings.TrimSpace(c.Query("n")); rawLimit != "" {
		parsed, err := strconv.Atoi(rawLimit)
		if err != nil || parsed < 0 {
			writeOCIError(c, http.StatusBadRequest, "UNSUPPORTED", "invalid tag limit")
			return
		}
		if parsed > 0 {
			limit = parsed
		}
	}

	tags, err := s.db.ListRepositoryTags(c.Request.Context(), registryID, repoLeaf(repo), limit, c.Query("last"))
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to list tags")
		return
	}

	c.JSON(http.StatusOK, tagListResponse{
		Name: repo,
		Tags: tags,
	})
}
