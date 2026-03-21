package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) listReferrersHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	subjectHex, err := parseDigest(digest)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}
	if s.registryDB == nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "manifest index unavailable")
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

	records, err := s.registryDB.ListRepositoryManifestRecords(c.Request.Context(), registryID, repoLeaf(repo))
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to list referrers")
		return
	}

	artifactType := strings.TrimSpace(c.Query("artifactType"))
	descriptors, err := buildReferrerDescriptors(records, "sha256:"+subjectHex, artifactType)
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to build referrers response")
		return
	}

	if artifactType != "" {
		c.Header("OCI-Filters-Applied", "artifactType")
	}

	body, err := json.Marshal(imageIndex{
		SchemaVersion: 2,
		MediaType:     defaultIndexContentType,
		Manifests:     descriptors,
	})
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to encode referrers response")
		return
	}
	c.Data(http.StatusOK, defaultIndexContentType, body)
}

func buildReferrerDescriptors(records []db.RepositoryManifestRecord, subjectDigest, artifactType string) ([]descriptor, error) {
	subjectDigest = strings.TrimSpace(subjectDigest)
	artifactType = strings.TrimSpace(artifactType)

	descriptors := make([]descriptor, 0, len(records))
	for _, record := range records {
		desc, recordSubjectDigest, ok, err := descriptorFromManifestRecord(record)
		if err != nil {
			return nil, err
		}
		if !ok || recordSubjectDigest != subjectDigest {
			continue
		}
		if artifactType != "" && desc.ArtifactType != artifactType {
			continue
		}
		descriptors = append(descriptors, desc)
	}

	slices.SortFunc(descriptors, func(a, b descriptor) int {
		return strings.Compare(a.Digest, b.Digest)
	})
	return descriptors, nil
}

func descriptorFromManifestRecord(record db.RepositoryManifestRecord) (descriptor, string, bool, error) {
	var manifest imageManifest
	if err := json.Unmarshal(record.Body, &manifest); err != nil {
		return descriptor{}, "", false, fmt.Errorf("parse manifest %s: %w", record.Digest, err)
	}

	if manifest.Subject == nil || strings.TrimSpace(manifest.Subject.Digest) == "" {
		return descriptor{}, "", false, nil
	}

	subjectHex, err := parseDigest(manifest.Subject.Digest)
	if err != nil {
		return descriptor{}, "", false, fmt.Errorf("parse manifest subject %s: %w", record.Digest, err)
	}

	mediaType := strings.TrimSpace(manifest.MediaType)
	if mediaType == "" {
		mediaType = manifestContentType(record.ContentType)
	}

	desc := descriptor{
		MediaType:   mediaType,
		Digest:      strings.TrimSpace(record.Digest),
		Size:        record.Size,
		Annotations: manifest.Annotations,
	}
	if artifactType := effectiveArtifactType(manifest); artifactType != "" {
		desc.ArtifactType = artifactType
	}

	return desc, "sha256:" + subjectHex, true, nil
}

func effectiveArtifactType(manifest imageManifest) string {
	if artifactType := strings.TrimSpace(manifest.ArtifactType); artifactType != "" {
		return artifactType
	}
	return strings.TrimSpace(manifest.Config.MediaType)
}
