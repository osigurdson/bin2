package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func writeOCIError(c *gin.Context, status int, code, message string) {
	c.JSON(status, ociErrorResponse{
		Errors: []ociError{
			{Code: code, Message: message},
		},
	})
}

func writeOCIUnauthorized(c *gin.Context) {
	c.Header("WWW-Authenticate", `Basic realm="registry"`)
	writeOCIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
}
