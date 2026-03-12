package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"bin2.io/internal/db"
	"github.com/gin-gonic/gin"
)

func (s *Server) startBlobUploadHandler(c *gin.Context, repo string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	if digest := strings.TrimSpace(c.Query("digest")); digest != "" && strings.TrimSpace(c.Query("mount")) == "" {
		s.monolithicBlobUploadHandler(c, repo, digest)
		return
	}

	if mountDigest := strings.TrimSpace(c.Query("mount")); mountDigest != "" {
		s.mountBlobHandler(c, repo, mountDigest, strings.TrimSpace(c.Query("from")))
		return
	}

	uuid, err := newUUID()
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload id")
		return
	}

	if err := s.registryStorage.CreateUpload(c.Request.Context(), uuid); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload")
		return
	}

	setUploadHeaders(c, repo, uuid, 0)
	c.Status(http.StatusAccepted)
}

func (s *Server) patchBlobUploadHandler(c *gin.Context, repo, uuid string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}
	if !reUUID.MatchString(uuid) {
		writeOCIError(c, http.StatusBadRequest, "BLOB_UPLOAD_INVALID", "invalid upload uuid")
		return
	}

	currentSize, err := s.registryStorage.UploadSize(c.Request.Context(), uuid)
	if errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to read upload status")
		return
	}

	if rawRange := strings.TrimSpace(c.GetHeader("Content-Range")); rawRange != "" {
		start, end, err := parseUploadRange(rawRange)
		if err != nil || start != currentSize || !uploadRangeMatchesContentLength(start, end, c.Request.ContentLength) {
			setUploadHeaders(c, repo, uuid, currentSize)
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	size, err := s.registryStorage.AppendUpload(c.Request.Context(), uuid, c.Request.Body)
	if errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to append upload")
		return
	}

	setUploadHeaders(c, repo, uuid, size)
	c.Status(http.StatusAccepted)
}

func (s *Server) getBlobUploadHandler(c *gin.Context, repo, uuid string) {
	if !validRepoName(repo) {
		c.Status(http.StatusBadRequest)
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}
	if !reUUID.MatchString(uuid) {
		c.Status(http.StatusBadRequest)
		return
	}

	size, err := s.registryStorage.UploadSize(c.Request.Context(), uuid)
	if errors.Is(err, ErrUploadNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	setUploadHeaders(c, repo, uuid, size)
	c.Status(http.StatusNoContent)
}

func (s *Server) finalizeBlobUploadHandler(c *gin.Context, repo, uuid string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}
	if !reUUID.MatchString(uuid) {
		writeOCIError(c, http.StatusBadRequest, "BLOB_UPLOAD_INVALID", "invalid upload uuid")
		return
	}

	digestHex, err := parseDigest(c.Query("digest"))
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}

	currentSize, err := s.registryStorage.UploadSize(c.Request.Context(), uuid)
	if errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to read upload status")
		return
	}

	if rawRange := strings.TrimSpace(c.GetHeader("Content-Range")); rawRange != "" {
		start, end, err := parseUploadRange(rawRange)
		if err != nil || start != currentSize || !uploadRangeMatchesContentLength(start, end, c.Request.ContentLength) {
			setUploadHeaders(c, repo, uuid, currentSize)
			c.Status(http.StatusRequestedRangeNotSatisfiable)
			return
		}
	}

	// Docker clients may send the final blob chunk in the same PUT request that
	// includes the digest query parameter. Append that body before hashing.
	body := c.Request.Body
	if body == nil {
		body = http.NoBody
	}
	if _, err := s.registryStorage.AppendUpload(c.Request.Context(), uuid, body); errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	} else if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to append upload")
		return
	}

	s.completeBlobUpload(c, repo, uuid, digestHex)
}

func (s *Server) monolithicBlobUploadHandler(c *gin.Context, repo, digest string) {
	digestHex, err := parseDigest(digest)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}

	uuid, err := newUUID()
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload id")
		return
	}
	if err := s.registryStorage.CreateUpload(c.Request.Context(), uuid); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload")
		return
	}

	body := c.Request.Body
	if body == nil {
		body = http.NoBody
	}
	if _, err := s.registryStorage.AppendUpload(c.Request.Context(), uuid, body); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to append upload")
		return
	}

	s.completeBlobUpload(c, repo, uuid, digestHex)
}

func (s *Server) mountBlobHandler(c *gin.Context, repo, mountDigest, fromRepo string) {
	digestHex, err := parseDigest(mountDigest)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}

	if fromRepo != "" && !validRepoName(fromRepo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid source repository name")
		return
	}

	size, err := s.registryStorage.BlobSize(c.Request.Context(), digestHex)
	if err == nil {
		digest := "sha256:" + digestHex
		if err := s.trackRegistryBlobDigest(c.Request.Context(), digest, size); err != nil {
			logError(fmt.Errorf("could not update registry blob index for %s: %w", digest, err))
		}
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", repo, digest))
		c.Header("Docker-Content-Digest", digest)
		c.Status(http.StatusCreated)
		return
	}
	if !errors.Is(err, ErrBlobNotFound) {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to check blob mount source")
		return
	}

	uuid, err := newUUID()
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload id")
		return
	}
	if err := s.registryStorage.CreateUpload(c.Request.Context(), uuid); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload")
		return
	}

	setUploadHeaders(c, repo, uuid, 0)
	c.Status(http.StatusAccepted)
}

func (s *Server) completeBlobUpload(c *gin.Context, repo, uuid, digestHex string) {
	computedHex, err := s.registryStorage.UploadSHA256(c.Request.Context(), uuid)
	if errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to hash upload")
		return
	}
	if computedHex != digestHex {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", "upload digest mismatch")
		return
	}

	digest := "sha256:" + digestHex
	size, exists, err := s.trackedRegistryBlobSize(c.Request.Context(), digest)
	if err != nil {
		logError(fmt.Errorf("could not read registry blob index for %s: %w", digest, err))
	}
	if exists {
		_ = s.registryStorage.DeleteUpload(c.Request.Context(), uuid)
		if err := s.trackRegistryBlobDigest(c.Request.Context(), digest, size); err != nil {
			logError(fmt.Errorf("could not update registry blob index for %s: %w", digest, err))
		}
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", repo, digest))
		c.Header("Docker-Content-Digest", digest)
		c.Status(http.StatusCreated)
		return
	}

	size, err = s.registryStorage.StoreBlobFromUpload(c.Request.Context(), uuid, digestHex)
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to finalize blob upload")
		return
	}
	if err := s.trackRegistryBlobDigest(c.Request.Context(), digest, size); err != nil {
		logError(fmt.Errorf("could not update registry blob index for %s: %w", digest, err))
	}

	c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", repo, digest))
	c.Header("Docker-Content-Digest", digest)
	c.Status(http.StatusCreated)
}

func setUploadHeaders(c *gin.Context, repo, uuid string, size int64) {
	c.Header("Location", fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uuid))
	c.Header("Docker-Upload-UUID", uuid)
	c.Header("Range", uploadRange(size))
}

func (s *Server) headBlobHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	digestHex, err := parseDigest(digest)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
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

	normalizedDigest := "sha256:" + digestHex
	size, err := s.db.GetRepositoryObjectSize(c.Request.Context(), registryID, repoLeaf(repo), normalizedDigest)
	if errors.Is(err, db.ErrNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UNKNOWN", "blob not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to resolve blob")
		return
	}

	s.noteObjectExistenceCheck(c.Request.Context(), normalizedDigest)

	c.Header("Docker-Content-Digest", normalizedDigest)
	c.Header("Content-Length", fmt.Sprintf("%d", size))
	c.Status(http.StatusOK)
}

func (s *Server) getBlobHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	digestHex, err := parseDigest(digest)
	if err != nil {
		writeOCIError(c, http.StatusBadRequest, "DIGEST_INVALID", err.Error())
		return
	}

	body, size, err := s.registryStorage.GetBlob(c.Request.Context(), digestHex)
	if errors.Is(err, ErrBlobNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UNKNOWN", "blob not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to load blob")
		return
	}
	defer body.Close()

	c.Header("Docker-Content-Digest", "sha256:"+digestHex)
	c.DataFromReader(http.StatusOK, size, defaultBlobContentType, body, nil)
}

func (s *Server) deleteBlobHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		writeOCIError(c, http.StatusBadRequest, "NAME_INVALID", "invalid repository name")
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}
	writeOCIError(c, http.StatusMethodNotAllowed, "UNSUPPORTED", "blob deletion is disabled")
}
