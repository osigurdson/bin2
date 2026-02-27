package server

import (
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func (s *Server) addRoutes() {
	s.router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	s.router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	s.addWellKnownRoutes()
	s.addRegistryRoutes()

	api := s.router.Group("/api/v1")
	api.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	registries := api.Group("/registries")
	registries.Use(s.authMiddleware())
	registries.GET("", s.listRegistriesHandler)
	registries.GET("/:id", s.getRegistryByIDHandler)
	registries.GET("/exists", s.getRegistryExistsHandler)
	registries.POST("", s.addRegistryHandler)

	apikeys := api.Group("/api-keys")
	apikeys.Use(s.authMiddleware())
	apikeys.POST("", s.addAPIKeyHandler)
	apikeys.GET("", s.listAPIKeysHandler)
	apikeys.DELETE(":id", s.removeAPIKeyHandler)
}
