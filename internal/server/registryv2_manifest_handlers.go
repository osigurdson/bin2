package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (s *Server) putManifestHandler(c *gin.Context, repo, reference string) {
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
	if !validReference(reference) {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "invalid manifest reference")
		return
	}

	manifestBytes, err := readManifestBody(c)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", err.Error())
		return
	}

	var manifest imageManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "invalid manifest JSON")
		return
	}

	blobDigests, childManifestDigests := extractManifestReferences(manifest)
	if len(blobDigests) == 0 && len(childManifestDigests) == 0 {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest must reference config/layer blobs")
		return
	}

	subjectDigest := ""
	if manifest.Subject != nil && manifest.Subject.Digest != "" {
		subjectHex, err := parseDigest(manifest.Subject.Digest)
		if err != nil {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest references invalid subject digest")
			return
		}
		subjectDigest = "sha256:" + subjectHex
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

	tenantID, err := s.db.GetRegistryTenantID(c.Request.Context(), registryID)
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to resolve tenant")
		return
	}

	normalizedBlobDigests := make([]string, 0, len(blobDigests))
	seenBlobDigests := make(map[string]struct{}, len(blobDigests))
	blobSizes := make(map[string]int64, len(blobDigests))
	for _, digest := range blobDigests {
		digestHex, err := parseDigest(digest)
		if err != nil {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest references invalid digest")
			return
		}
		normalizedDigest := "sha256:" + digestHex
		if _, ok := seenBlobDigests[normalizedDigest]; ok {
			continue
		}
		size, err := s.registryStorage.BlobSize(c.Request.Context(), digestHex)
		if errors.Is(err, ErrBlobNotFound) {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_BLOB_UNKNOWN", "referenced blob not found")
			return
		}
		if err != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to validate referenced blob")
			return
		}

		seenBlobDigests[normalizedDigest] = struct{}{}
		normalizedBlobDigests = append(normalizedBlobDigests, normalizedDigest)
		blobSizes[normalizedDigest] = size
	}

	for _, blobDigest := range normalizedBlobDigests {
		if err := s.trackRegistryBlobDigest(c.Request.Context(), blobDigest, blobSizes[blobDigest]); err != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to index referenced blob")
			return
		}
	}

	normalizedChildManifestDigests := make([]string, 0, len(childManifestDigests))
	seenManifestDigests := make(map[string]struct{}, len(childManifestDigests))
	for _, childManifestDigest := range childManifestDigests {
		if s.db == nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "manifest index unavailable")
			return
		}
		digestHex, err := parseDigest(childManifestDigest)
		if err != nil {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest references invalid digest")
			return
		}
		normalizedDigest := "sha256:" + digestHex
		if _, ok := seenManifestDigests[normalizedDigest]; ok {
			continue
		}
		exists, err := s.db.HasManifestDigestInRepository(c.Request.Context(), registryID, repoLeaf(repo), normalizedDigest)
		if err != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to validate referenced manifest")
			return
		}
		if !exists {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_BLOB_UNKNOWN", "referenced manifest not found")
			return
		}
		seenManifestDigests[normalizedDigest] = struct{}{}
		normalizedChildManifestDigests = append(normalizedChildManifestDigests, normalizedDigest)
	}

	sum := sha256.Sum256(manifestBytes)
	manifestDigest := "sha256:" + hex.EncodeToString(sum[:])

	tag := ""
	if reference != manifestDigest {
		tag = reference
	}

	contentType := manifestContentType(c.GetHeader("Content-Type"))
	if err := s.indexRegistryManifest(
		c.Request.Context(),
		registryID,
		repoLeaf(repo),
		manifestDigest,
		manifestBytes,
		contentType,
		tag,
		normalizedBlobDigests,
		normalizedChildManifestDigests,
		subjectDigest,
	); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to store manifest")
		return
	}

	// Emit push-op-count for the manifest itself.
	s.emitUsageEvent(c.Request.Context(), tenantID, registryID, nil, db.MetricPushOpCount, 1)

	c.Header("Docker-Content-Digest", manifestDigest)
	if subjectDigest != "" {
		c.Header("OCI-Subject", subjectDigest)
	}
	c.Header("Location", "/v2/"+repo+"/manifests/"+reference)
	c.Status(http.StatusCreated)
}

func (s *Server) getManifestHandler(c *gin.Context, repo, reference string) {
	manifestBytes, contentType, digest, err := s.loadManifestResponse(c, repo, reference)
	if err != nil {
		return
	}

	c.Header("Docker-Content-Digest", digest)
	c.Data(http.StatusOK, contentType, manifestBytes)
}

func (s *Server) headManifestHandler(c *gin.Context, repo, reference string) {
	manifestBytes, contentType, digest, err := s.loadManifestResponse(c, repo, reference)
	if err != nil {
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Docker-Content-Digest", digest)
	c.Header("Content-Length", fmt.Sprintf("%d", len(manifestBytes)))
	c.Status(http.StatusOK)
}

func (s *Server) deleteManifestHandler(c *gin.Context, repo, reference string) {
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
	if !validReference(reference) {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "invalid manifest reference")
		return
	}
	if s.db == nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "manifest index unavailable")
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

	tenantID, tenantErr := s.db.GetRegistryTenantID(c.Request.Context(), registryID)
	if tenantErr != nil {
		logError(fmt.Errorf("GetRegistryTenantID: %w", tenantErr))
		tenantID = uuid.Nil
	}

	var deleted bool
	var deleteErr error
	if digestHex, err := parseDigest(reference); err == nil {
		var orphaned []db.DeletedBlobInfo
		deleted, orphaned, deleteErr = s.db.DeleteManifestByDigestInRepository(
			c.Request.Context(),
			registryID,
			tenantID,
			repoLeaf(repo),
			"sha256:"+digestHex,
		)
		if deleteErr != nil {
			if errors.Is(deleteErr, db.ErrManifestHasParent) {
				writeOCIError(c, http.StatusConflict, "MANIFEST_REFERENCED", "manifest is referenced by an index; delete the index first")
				return
			}
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to delete manifest")
			return
		}
		// Emit negative storage-bytes for blobs now orphaned at tenant level.
		for _, blob := range orphaned {
			s.emitUsageEvent(c.Request.Context(), tenantID, registryID, nil, db.MetricStorageBytes, -blob.SizeBytes)
		}
	} else {
		deleted, deleteErr = s.db.DeleteManifestReference(
			c.Request.Context(),
			registryID,
			repoLeaf(repo),
			reference,
		)
		if deleteErr != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to delete manifest")
			return
		}
	}
	if !deleted {
		c.Status(http.StatusNotFound)
		return
	}

	c.Status(http.StatusAccepted)
}

func (s *Server) loadManifestResponse(c *gin.Context, repo, reference string) ([]byte, string, string, error) {
	if !validRepoName(repo) {
		c.Status(http.StatusBadRequest)
		return nil, "", "", errors.New("invalid repo")
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return nil, "", "", errors.New("forbidden")
	}
	auth, err := s.getRegistryAuth(c)
	if err != nil {
		c.Status(http.StatusUnauthorized)
		return nil, "", "", err
	}
	if !validReference(reference) {
		c.Status(http.StatusBadRequest)
		return nil, "", "", errors.New("invalid reference")
	}

	registryID, err := s.resolveRegistryIDForRepo(c.Request.Context(), auth, repo)
	if err != nil {
		if errors.Is(err, errUnauthorized) {
			c.Status(http.StatusForbidden)
			return nil, "", "", err
		}
		c.Status(http.StatusInternalServerError)
		return nil, "", "", err
	}

	manifestBytes, contentType, digest, err := s.db.GetManifestByReference(
		c.Request.Context(),
		registryID,
		repoLeaf(repo),
		reference,
	)
	if errors.Is(err, db.ErrNotFound) {
		c.Status(http.StatusNotFound)
		return nil, "", "", err
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return nil, "", "", err
	}
	return manifestBytes, manifestContentType(contentType), digest, nil
}

func extractManifestBlobDigests(m imageManifest) []string {
	out := make([]string, 0, 1+len(m.Layers))
	if m.Config.Digest != "" {
		out = append(out, m.Config.Digest)
	}
	for _, layer := range m.Layers {
		if layer.Digest != "" {
			out = append(out, layer.Digest)
		}
	}
	return out
}

func extractManifestReferences(m imageManifest) ([]string, []string) {
	if len(m.Manifests) > 0 {
		out := make([]string, 0, len(m.Manifests))
		for _, child := range m.Manifests {
			if child.Digest != "" {
				out = append(out, child.Digest)
			}
		}
		return nil, out
	}
	return extractManifestBlobDigests(m), nil
}
