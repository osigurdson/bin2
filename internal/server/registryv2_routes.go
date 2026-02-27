package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) addRegistryRoutes() {
	v2Middlewares := []gin.HandlerFunc{
		s.apiVersionMiddleware(),
		s.registryBearerAuthMiddleware(),
	}
	s.router.Any("/v2", append(v2Middlewares, s.v2RootHandler)...)

	v2 := s.router.Group("/v2")
	v2.Use(v2Middlewares...)
	v2.Any("/*path", s.v2Handler)
}

func (s *Server) v2RootHandler(c *gin.Context) {
	if c.Request.Method == http.MethodGet {
		c.Status(http.StatusOK)
		return
	}
	writeOCIError(c, http.StatusMethodNotAllowed, "UNSUPPORTED", "method not allowed")
}

func (s *Server) v2Handler(c *gin.Context) {
	relative := strings.TrimPrefix(c.Param("path"), "/")
	if relative == "" || relative == "/" {
		s.v2RootHandler(c)
		return
	}
	if isRegistryTokenPath(relative) {
		s.registryTokenHandler(c)
		return
	}

	if c.Request.Method == http.MethodPost {
		if m := reStartUpload.FindStringSubmatch(relative); m != nil {
			s.startBlobUploadHandler(c, m[1])
			return
		}
	}

	if c.Request.Method == http.MethodPatch || c.Request.Method == http.MethodPut {
		if m := reUploadChunk.FindStringSubmatch(relative); m != nil {
			repo := m[1]
			uuid := m[2]
			if c.Request.Method == http.MethodPatch {
				s.patchBlobUploadHandler(c, repo, uuid)
				return
			}
			s.finalizeBlobUploadHandler(c, repo, uuid)
			return
		}
	}

	if c.Request.Method == http.MethodHead || c.Request.Method == http.MethodGet {
		if m := reBlobPath.FindStringSubmatch(relative); m != nil {
			if c.Request.Method == http.MethodHead {
				s.headBlobHandler(c, m[1], m[2])
				return
			}
			s.getBlobHandler(c, m[1], m[2])
			return
		}
	}

	if c.Request.Method == http.MethodPut || c.Request.Method == http.MethodHead || c.Request.Method == http.MethodGet {
		if m := reManifestRef.FindStringSubmatch(relative); m != nil {
			switch c.Request.Method {
			case http.MethodPut:
				s.putManifestHandler(c, m[1], m[2])
			case http.MethodHead:
				s.headManifestHandler(c, m[1], m[2])
			default:
				s.getManifestHandler(c, m[1], m[2])
			}
			return
		}
	}

	writeOCIError(c, http.StatusNotFound, "UNSUPPORTED", "endpoint not implemented")
}
