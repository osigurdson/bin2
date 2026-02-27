package server

import (
	"errors"
	"fmt"
	"net/http"

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

	uuid, err := newUUID()
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload id")
		return
	}

	if err := s.registryStorage.CreateUpload(c.Request.Context(), uuid); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to create upload")
		return
	}

	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uuid)
	c.Header("Location", location)
	c.Header("Docker-Upload-UUID", uuid)
	c.Header("Range", "0-0")
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

	size, err := s.registryStorage.AppendUpload(c.Request.Context(), uuid, c.Request.Body)
	if errors.Is(err, ErrUploadNotFound) {
		writeOCIError(c, http.StatusNotFound, "BLOB_UPLOAD_UNKNOWN", "upload not found")
		return
	}
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to append upload")
		return
	}

	location := fmt.Sprintf("/v2/%s/blobs/uploads/%s", repo, uuid)
	c.Header("Location", location)
	c.Header("Docker-Upload-UUID", uuid)
	c.Header("Range", uploadRange(size))
	c.Status(http.StatusAccepted)
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
	exists, err := s.registryStorage.BlobExists(c.Request.Context(), digestHex)
	if err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to check blob existence")
		return
	}
	if exists {
		_ = s.registryStorage.DeleteUpload(c.Request.Context(), uuid)
		c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", repo, digest))
		c.Header("Docker-Content-Digest", digest)
		c.Status(http.StatusCreated)
		return
	}

	if err := s.registryStorage.StoreBlobFromUpload(c.Request.Context(), uuid, digestHex); err != nil {
		writeOCIError(c, http.StatusInternalServerError, "UNKNOWN", "failed to finalize blob upload")
		return
	}

	c.Header("Location", fmt.Sprintf("/v2/%s/blobs/%s", repo, digest))
	c.Header("Docker-Content-Digest", digest)
	c.Status(http.StatusCreated)
}

func (s *Server) headBlobHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		c.Status(http.StatusBadRequest)
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	digestHex, err := parseDigest(digest)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	size, err := s.registryStorage.BlobSize(c.Request.Context(), digestHex)
	if errors.Is(err, ErrBlobNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Header("Docker-Content-Digest", "sha256:"+digestHex)
	c.Header("Content-Length", fmt.Sprintf("%d", size))
	c.Status(http.StatusOK)
}

func (s *Server) getBlobHandler(c *gin.Context, repo, digest string) {
	if !validRepoName(repo) {
		c.Status(http.StatusBadRequest)
		return
	}
	if !s.ensureRepoAuthorized(c, repo) {
		return
	}

	digestHex, err := parseDigest(digest)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	body, size, err := s.registryStorage.GetBlob(c.Request.Context(), digestHex)
	if errors.Is(err, ErrBlobNotFound) {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	defer body.Close()

	c.Header("Docker-Content-Digest", "sha256:"+digestHex)
	c.DataFromReader(http.StatusOK, size, defaultBlobContentType, body, nil)
}
