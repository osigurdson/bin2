package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) putManifestHandler(c *gin.Context, repo, reference string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
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

	blobDigests := extractManifestBlobDigests(manifest)
	if len(blobDigests) == 0 {
		writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest must reference config/layer blobs")
		return
	}

	for _, digest := range blobDigests {
		digestHex, err := parseDigest(digest)
		if err != nil {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_INVALID", "manifest references invalid digest")
			return
		}
		exists, err := s.registryStorage.BlobExists(c.Request.Context(), digestHex)
		if err != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to validate referenced blob")
			return
		}
		if !exists {
			writeOCIError(c, http.StatusBadRequest, "MANIFEST_BLOB_UNKNOWN", "referenced blob not found")
			return
		}
	}

	contentType := manifestContentType(c.GetHeader("Content-Type"))
	if err := s.registryStorage.StoreManifest(c.Request.Context(), repo, reference, manifestBytes, contentType); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to store manifest")
		return
	}

	sum := sha256.Sum256(manifestBytes)
	manifestDigest := "sha256:" + hex.EncodeToString(sum[:])
	if reference != manifestDigest {
		if err := s.registryStorage.StoreManifest(c.Request.Context(), repo, manifestDigest, manifestBytes, contentType); err != nil {
			writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to store digest manifest")
			return
		}
	}

	c.Header("Docker-Content-Digest", manifestDigest)
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

func (s *Server) loadManifestResponse(c *gin.Context, repo, reference string) ([]byte, string, string, error) {
	if !validRepoName(repo) {
		c.Status(http.StatusBadRequest)
		return nil, "", "", errors.New("invalid repo")
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return nil, "", "", errors.New("forbidden")
	}
	if !validReference(reference) {
		c.Status(http.StatusBadRequest)
		return nil, "", "", errors.New("invalid reference")
	}

	manifestBytes, contentType, err := s.registryStorage.GetManifest(c.Request.Context(), repo, reference)
	if errors.Is(err, ErrManifestNotFound) {
		c.Status(http.StatusNotFound)
		return nil, "", "", err
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return nil, "", "", err
	}

	sum := sha256.Sum256(manifestBytes)
	digest := "sha256:" + hex.EncodeToString(sum[:])
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
