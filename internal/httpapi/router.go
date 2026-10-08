package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kitchu-studio/booking-demo-backend/internal/auth"
	"github.com/kitchu-studio/booking-demo-backend/internal/store"
)

type Server struct {
	store  *store.Store
	tokens *auth.TokenManager
}

func NewRouter(st *store.Store, tokens *auth.TokenManager, appEnv string) *gin.Engine {
	if appEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{store: st, tokens: tokens}

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	r.GET("/healthz", s.health)

	v1 := r.Group("/api/v1")
	{
		a := v1.Group("/auth")
		a.POST("/register", s.register)
		a.POST("/login", s.login)
		a.POST("/refresh", s.refresh)

		authed := v1.Group("")
		authed.Use(s.requireAuth())
		authed.GET("/me", s.me)
	}
	return r
}

func (s *Server) health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
