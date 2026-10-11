package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kitchu-studio/booking-demo-backend/internal/schedule"
)

const exclusionViolation = "23P01"

var ErrSlotTaken = errors.New("time slot already booked")

type Service struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	DurationMinutes int       `json:"duration_minutes"`
	PriceBaht       int       `json:"price_baht"`
	CreatedAt       time.Time `json:"-"`
}

type Booking struct {
	ID          int64      `json:"id"`
	ServiceID   int64      `json:"service_id"`
	ServiceName string     `json:"service_name"`
	StartsAt    time.Time  `json:"starts_at"`
	EndsAt      time.Time  `json:"ends_at"`
	Status      string     `json:"status"`
	Note        string     `json:"note"`
	CreatedAt   time.Time  `json:"created_at"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
}

func (s *Store) ListServices(ctx context.Context) ([]Service, error) {
	const q = `
		SELECT id, name, description, duration_minutes, price_baht, created_at
		FROM services WHERE active ORDER BY id`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Service{}
	for rows.Next() {
		var sv Service
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.Description, &sv.DurationMinutes, &sv.PriceBaht, &sv.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, sv)
	}
	return out, rows.Err()
}

func (s *Store) GetService(ctx context.Context, id int64) (*Service, error) {
	const q = `
		SELECT id, name, description, duration_minutes, price_baht, created_at
		FROM services WHERE id = $1 AND active`
	var sv Service
	err := s.pool.QueryRow(ctx, q, id).
		Scan(&sv.ID, &sv.Name, &sv.Description, &sv.DurationMinutes, &sv.PriceBaht, &sv.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &sv, nil
}

// BusyRanges returns confirmed bookings that overlap [from, to).
func (s *Store) BusyRanges(ctx context.Context, from, to time.Time) ([]schedule.Range, error) {
	const q = `
		SELECT starts_at, ends_at FROM bookings
		WHERE status = 'confirmed' AND starts_at < $2 AND ends_at > $1
		ORDER BY starts_at`
	rows, err := s.pool.Query(ctx, q, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []schedule.Range{}
	for rows.Next() {
		var r schedule.Range
		if err := rows.Scan(&r.Start, &r.End); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// CreateBooking inserts a confirmed booking. The database exclusion constraint
// guarantees no overlap, and the violation is reported as ErrSlotTaken.
func (s *Store) CreateBooking(ctx context.Context, userID int64, svc *Service, start, end time.Time, note string) (*Booking, error) {
	const q = `
		INSERT INTO bookings (user_id, service_id, starts_at, ends_at, note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, starts_at, ends_at, status, note, created_at`
	b := Booking{ServiceID: svc.ID, ServiceName: svc.Name}
	err := s.pool.QueryRow(ctx, q, userID, svc.ID, start, end, note).
		Scan(&b.ID, &b.StartsAt, &b.EndsAt, &b.Status, &b.Note, &b.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == exclusionViolation {
			return nil, ErrSlotTaken
		}
		return nil, err
	}
	return &b, nil
}

func (s *Store) ListBookingsByUser(ctx context.Context, userID int64) ([]Booking, error) {
	const q = `
		SELECT b.id, b.service_id, sv.name, b.starts_at, b.ends_at, b.status, b.note, b.created_at, b.cancelled_at
		FROM bookings b
		JOIN services sv ON sv.id = b.service_id
		WHERE b.user_id = $1
		ORDER BY b.starts_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []Booking{}
	for rows.Next() {
		var b Booking
		if err := rows.Scan(&b.ID, &b.ServiceID, &b.ServiceName, &b.StartsAt, &b.EndsAt,
			&b.Status, &b.Note, &b.CreatedAt, &b.CancelledAt); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// CancelBooking cancels a confirmed booking owned by userID.
// Returns ErrNotFound if it does not exist, is not theirs, or is already cancelled.
func (s *Store) CancelBooking(ctx context.Context, id, userID int64) (*Booking, error) {
	const q = `
		WITH upd AS (
			UPDATE bookings
			SET status = 'cancelled', cancelled_at = now()
			WHERE id = $1 AND user_id = $2 AND status = 'confirmed'
			RETURNING id, service_id, starts_at, ends_at, status, note, created_at, cancelled_at
		)
		SELECT upd.id, upd.service_id, sv.name, upd.starts_at, upd.ends_at, upd.status, upd.note, upd.created_at, upd.cancelled_at
		FROM upd JOIN services sv ON sv.id = upd.service_id`
	var b Booking
	err := s.pool.QueryRow(ctx, q, id, userID).
		Scan(&b.ID, &b.ServiceID, &b.ServiceName, &b.StartsAt, &b.EndsAt, &b.Status, &b.Note, &b.CreatedAt, &b.CancelledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}
