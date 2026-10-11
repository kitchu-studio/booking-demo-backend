CREATE TABLE services (
    id               BIGSERIAL PRIMARY KEY,
    name             TEXT        NOT NULL,
    description      TEXT        NOT NULL DEFAULT '',
    duration_minutes INT         NOT NULL CHECK (duration_minutes > 0),
    price_baht       INT         NOT NULL CHECK (price_baht >= 0),
    active           BOOLEAN     NOT NULL DEFAULT true,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bookings (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    service_id   BIGINT      NOT NULL REFERENCES services (id),
    starts_at    TIMESTAMPTZ NOT NULL,
    ends_at      TIMESTAMPTZ NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'confirmed' CHECK (status IN ('confirmed', 'cancelled')),
    note         TEXT        NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    cancelled_at TIMESTAMPTZ,
    CHECK (ends_at > starts_at),
    -- One shop / one staff: two confirmed bookings can never overlap.
    -- Enforced by the database, so it holds even with concurrent requests.
    CONSTRAINT bookings_no_overlap
        EXCLUDE USING gist (tstzrange(starts_at, ends_at) WITH &&)
        WHERE (status = 'confirmed')
);

CREATE INDEX bookings_user_idx ON bookings (user_id, starts_at DESC);
