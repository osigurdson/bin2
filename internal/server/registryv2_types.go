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
	MediaType    string            `json:"mediaType,omitempty"`
	Digest       string            `json:"digest"`
	Size         int64             `json:"size,omitempty"`
	ArtifactType string            `json:"artifactType,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
}

type imageManifest struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	ArtifactType  string            `json:"artifactType,omitempty"`
	Config        descriptor        `json:"config"`
	Layers        []descriptor      `json:"layers"`
	Manifests     []descriptor      `json:"manifests"`
	Subject       *descriptor       `json:"subject,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type imageIndex struct {
	SchemaVersion int               `json:"schemaVersion"`
	MediaType     string            `json:"mediaType,omitempty"`
	ArtifactType  string            `json:"artifactType,omitempty"`
	Manifests     []descriptor      `json:"manifests"`
	Subject       *descriptor       `json:"subject,omitempty"`
	Annotations   map[string]string `json:"annotations,omitempty"`
}

type tagListResponse struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

const (
	defaultBlobContentType     = "application/octet-stream"
	defaultIndexContentType    = "application/vnd.oci.image.index.v1+json"
	defaultManifestContentType = "application/vnd.oci.image.manifest.v1+json"
)

var (
	reStartUpload = regexp.MustCompile(`^(.+)/blobs/uploads/$`)
	reUploadChunk = regexp.MustCompile(`^(.+)/blobs/uploads/([^/]+)$`)
	reBlobPath    = regexp.MustCompile(`^(.+)/blobs/([^/]+)$`)
	reManifestRef = regexp.MustCompile(`^(.+)/manifests/([^/]+)$`)
	reTagsList    = regexp.MustCompile(`^(.+)/tags/list$`)
	reReferrers   = regexp.MustCompile(`^(.+)/referrers/([^/]+)$`)
	reDigest      = regexp.MustCompile(`^sha256:([a-fA-F0-9]{64})$`)
	reRepoSeg     = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	reUUID        = regexp.MustCompile(`^[a-f0-9-]{36}$`)
	reRange       = regexp.MustCompile(`^(\d+)-(\d+)$`)
)
