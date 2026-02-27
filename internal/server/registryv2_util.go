package server

import (
	"fmt"
	"io"
	"strings"

	"github.com/gin-gonic/gin"
)

func readManifestBody(c *gin.Context) ([]byte, error) {
	manifestBytes, err := io.ReadAll(io.LimitReader(c.Request.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest body")
	}
	return manifestBytes, nil
}

func manifestContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return defaultManifestContentType
	}
	parts := strings.Split(contentType, ";")
	mediaType := strings.TrimSpace(parts[0])
	if mediaType == "" {
		return defaultManifestContentType
	}
	return mediaType
}

func registryNamespace(repo string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return ""
	}
	idx := strings.Index(repo, "/")
	if idx == -1 {
		return repo
	}
	return repo[:idx]
}

// repoLeaf strips the registry namespace prefix from a full OCI repository
// path, returning only the portion stored in api_key_scopes.
// e.g. "myregistry/group/app" → "group/app"
func repoLeaf(repo string) string {
	idx := strings.Index(repo, "/")
	if idx == -1 {
		return repo
	}
	return repo[idx+1:]
}

func validRepoName(repo string) bool {
	if repo == "" || strings.Contains(repo, "..") {
		return false
	}
	parts := strings.Split(repo, "/")
	for _, part := range parts {
		if part == "" || !reRepoSeg.MatchString(part) {
			return false
		}
	}
	return true
}

func validReference(reference string) bool {
	if reference == "" {
		return false
	}
	if strings.Contains(reference, "/") || strings.Contains(reference, `\\`) {
		return false
	}
	if reference == "." || reference == ".." {
		return false
	}
	return true
}

func parseDigest(digest string) (string, error) {
	m := reDigest.FindStringSubmatch(digest)
	if m == nil {
		return "", fmt.Errorf("digest must be sha256:<64-hex>")
	}
	return strings.ToLower(m[1]), nil
}

func uploadRange(size int64) string {
	if size <= 0 {
		return "0-0"
	}
	return fmt.Sprintf("0-%d", size-1)
}
