package server

import "regexp"

type ociError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ociErrorResponse struct {
	Errors []ociError `json:"errors"`
}

type descriptor struct {
	Digest string `json:"digest"`
}

type imageManifest struct {
	SchemaVersion int          `json:"schemaVersion"`
	Config        descriptor   `json:"config"`
	Layers        []descriptor `json:"layers"`
}

const (
	defaultBlobContentType     = "application/octet-stream"
	defaultManifestContentType = "application/vnd.oci.image.manifest.v1+json"
)

var (
	reStartUpload = regexp.MustCompile(`^(.+)/blobs/uploads/$`)
	reUploadChunk = regexp.MustCompile(`^(.+)/blobs/uploads/([^/]+)$`)
	reBlobPath    = regexp.MustCompile(`^(.+)/blobs/([^/]+)$`)
	reManifestRef = regexp.MustCompile(`^(.+)/manifests/([^/]+)$`)
	reDigest      = regexp.MustCompile(`^sha256:([a-fA-F0-9]{64})$`)
	reRepoSeg     = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	reUUID        = regexp.MustCompile(`^[a-f0-9-]{36}$`)
)
