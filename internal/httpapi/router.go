package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kitchu-studio/booking-demo-backend/internal/auth"
	"github.com/kitchu-studio/booking-demo-backend/internal/schedule"
	"github.com/kitchu-studio/booking-demo-backend/internal/store"
)

type Server struct {
	store  *store.Store
	tokens *auth.TokenManager
	sched  *schedule.Schedule
}

func NewRouter(st *store.Store, tokens *auth.TokenManager, sched *schedule.Schedule, appEnv string) *gin.Engine {
	if appEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{store: st, tokens: tokens, sched: sched}

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	r.GET("/healthz", s.health)

	v1 := r.Group("/api/v1")
	{
		a := v1.Group("/auth")
		a.POST("/register", s.register)
		a.POST("/login", s.login)
		a.POST("/refresh", s.refresh)

		// Public: anyone can browse services and free times.
		v1.GET("/services", s.listServices)
		v1.GET("/services/:id/slots", s.listSlots)

		authed := v1.Group("")
		authed.Use(s.requireAuth())
		authed.GET("/me", s.me)
		authed.POST("/bookings", s.createBooking)
		authed.GET("/bookings", s.listMyBookings)
		authed.POST("/bookings/:id/cancel", s.cancelBooking)
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
