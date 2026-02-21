package server

import (
	"fmt"
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

func writeOCIUnauthorizedBearer(c *gin.Context, realm, service, scope string) {
	setBearerAuthChallenge(c, realm, service, scope)
	writeOCIError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
}

func setBearerAuthChallenge(c *gin.Context, realm, service, scope string) {
	challenge := fmt.Sprintf(`Bearer realm=%q,service=%q`, realm, service)
	if scope != "" {
		challenge += fmt.Sprintf(`,scope=%q`, scope)
	}
	c.Header("WWW-Authenticate", challenge)
}
