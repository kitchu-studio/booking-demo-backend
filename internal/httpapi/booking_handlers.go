package httpapi

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kitchu-studio/booking-demo-backend/internal/store"
)

const maxDaysAhead = 60

type createBookingRequest struct {
	ServiceID int64     `json:"service_id" binding:"required"`
	StartsAt  time.Time `json:"starts_at" binding:"required"`
	Note      string    `json:"note" binding:"max=300"`
}

type slotView struct {
	StartsAt string `json:"starts_at"`
	EndsAt   string `json:"ends_at"`
}

func (s *Server) listServices(c *gin.Context) {
	services, err := s.store.ListServices(c.Request.Context())
	if err != nil {
		slog.Error("list services", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, services)
}

// GET /api/v1/services/:id/slots?date=YYYY-MM-DD
func (s *Server) listSlots(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}

	svc, err := s.store.GetService(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			abort(c, http.StatusNotFound, "service not found")
			return
		}
		slog.Error("get service", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}

	day, err := s.sched.ParseDate(c.Query("date"))
	if err != nil {
		abort(c, http.StatusBadRequest, "date is required in YYYY-MM-DD format")
		return
	}
	now := time.Now()
	today, _ := s.sched.DayBounds(now)
	if day.Before(today) || day.After(today.AddDate(0, 0, maxDaysAhead)) {
		abort(c, http.StatusBadRequest, "date must be between today and "+strconv.Itoa(maxDaysAhead)+" days ahead")
		return
	}

	from, to := s.sched.DayBounds(day)
	busy, err := s.store.BusyRanges(c.Request.Context(), from, to)
	if err != nil {
		slog.Error("busy ranges", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}

	dur := time.Duration(svc.DurationMinutes) * time.Minute
	slots := make([]slotView, 0)
	for _, t := range s.sched.Slots(day, dur, busy, now) {
		slots = append(slots, slotView{
			StartsAt: t.Format(time.RFC3339),
			EndsAt:   t.Add(dur).Format(time.RFC3339),
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"date":             day.Format("2006-01-02"),
		"service_id":       svc.ID,
		"duration_minutes": svc.DurationMinutes,
		"timezone":         s.sched.Loc.String(),
		"slots":            slots,
	})
}

func (s *Server) createBooking(c *gin.Context) {
	var req createBookingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		abort(c, http.StatusBadRequest, "invalid request: service_id and starts_at (RFC3339) are required")
		return
	}

	svc, err := s.store.GetService(c.Request.Context(), req.ServiceID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			abort(c, http.StatusNotFound, "service not found")
			return
		}
		slog.Error("get service", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}

	dur := time.Duration(svc.DurationMinutes) * time.Minute
	if !req.StartsAt.After(time.Now()) {
		abort(c, http.StatusBadRequest, "starts_at must be in the future")
		return
	}
	if !s.sched.ValidStart(req.StartsAt, dur) {
		abort(c, http.StatusBadRequest, "starts_at must be on the slot grid and inside opening hours")
		return
	}

	userID := c.GetInt64(ctxUserID)
	booking, err := s.store.CreateBooking(c.Request.Context(), userID, svc,
		req.StartsAt, req.StartsAt.Add(dur), strings.TrimSpace(req.Note))
	if err != nil {
		if errors.Is(err, store.ErrSlotTaken) {
			abort(c, http.StatusConflict, "that time slot is no longer available")
			return
		}
		slog.Error("create booking", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusCreated, booking)
}

func (s *Server) listMyBookings(c *gin.Context) {
	bookings, err := s.store.ListBookingsByUser(c.Request.Context(), c.GetInt64(ctxUserID))
	if err != nil {
		slog.Error("list bookings", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, bookings)
}

func (s *Server) cancelBooking(c *gin.Context) {
	id, ok := parseID(c, "id")
	if !ok {
		return
	}
	booking, err := s.store.CancelBooking(c.Request.Context(), id, c.GetInt64(ctxUserID))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			abort(c, http.StatusNotFound, "booking not found or already cancelled")
			return
		}
		slog.Error("cancel booking", "error", err)
		abort(c, http.StatusInternalServerError, "internal error")
		return
	}
	c.JSON(http.StatusOK, booking)
}

func parseID(c *gin.Context, name string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		abort(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}
