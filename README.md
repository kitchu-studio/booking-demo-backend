# Booking Demo Backend

Go (Gin) + PostgreSQL starter for freelance projects. JWT auth (access + refresh),
SQL migrations, Docker Compose with Caddy (automatic HTTPS), GitHub Actions CI and a
manual deploy workflow for AWS EC2.

## Structure

```
cmd/api/              entrypoint
internal/config/      env config
internal/auth/        password hashing + JWT
internal/store/       database access (pgx)
internal/schedule/    opening hours, slot grid, overlap rules (unit tested)
internal/httpapi/     routes, handlers, middleware
migrations/           SQL migrations (golang-migrate format)
Dockerfile, docker-compose.yml, Caddyfile
.github/workflows/    ci.yml, deploy.yml
```

## First-time setup

```bash
go mod tidy                 # creates go.sum - commit it
cp .env.example .env        # then edit every value
docker compose up -d --build
curl -k https://localhost/healthz
```

## API

| Method | Path                           | Auth   | Body / query                                   |
|--------|--------------------------------|--------|------------------------------------------------|
| GET    | /healthz                       | no     |                                                |
| POST   | /api/v1/auth/register          | no     | `{email, password, name?}`                     |
| POST   | /api/v1/auth/login             | no     | `{email, password}`                            |
| POST   | /api/v1/auth/refresh           | no     | `{refresh_token}`                              |
| GET    | /api/v1/services               | no     | list bookable services                         |
| GET    | /api/v1/services/:id/slots     | no     | `?date=YYYY-MM-DD` free start times that day   |
| GET    | /api/v1/me                     | Bearer | current user                                   |
| POST   | /api/v1/bookings               | Bearer | `{service_id, starts_at (RFC3339), note?}`     |
| GET    | /api/v1/bookings               | Bearer | my bookings, newest first                      |
| POST   | /api/v1/bookings/:id/cancel    | Bearer | cancel my booking                              |

Booking rules: one shop / one staff, so two confirmed bookings can never overlap
(enforced by a PostgreSQL exclusion constraint, safe under concurrent requests).
Opening hours, slot size and timezone come from `OPEN_HOUR`, `CLOSE_HOUR`,
`SLOT_MINUTES`, `BUSINESS_TZ`. Times are returned with the shop's UTC offset
(e.g. `2026-10-12T10:00:00+07:00`). Booking window: today up to 60 days ahead.
A taken slot returns HTTP 409.

Try it:

```bash
# register (returns access_token)
curl -k -X POST https://localhost/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"demo@example.com","password":"longpassword","name":"Demo"}'

# list services and free times
curl -k https://localhost/api/v1/services
curl -k "https://localhost/api/v1/services/1/slots?date=2026-10-20"

# book (use a starts_at from the slots response)
curl -k -X POST https://localhost/api/v1/bookings \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"service_id":1,"starts_at":"2026-10-20T10:00:00+07:00"}'
```

Run unit tests: `go test ./...`

## Migrations

Run automatically by the `migrate` service on every `docker compose up`.
Create a new one: `make migrate-new name=create_bookings`.

## Deploy to the customer's EC2 (AWS account owned by the customer)

1. EC2 (Ubuntu/Amazon Linux) with Docker + compose plugin, SSM agent, and an IAM instance
   role that allows SSM and ECR pull. Security group: open 80/443 only, no public 22.
2. Copy `docker-compose.yml`, `Caddyfile`, `migrations/` and a real `.env` to `/opt/app`.
   Point the domain's DNS at the instance and set `DOMAIN` in `.env`.
3. Create an ECR repository and an IAM role for GitHub OIDC that can push to ECR and
   run `ssm:SendCommand` on the instance.
4. In the GitHub repo set Variables: `AWS_REGION`, `ECR_REPOSITORY`, `AWS_DEPLOY_ROLE_ARN`,
   `EC2_INSTANCE_ID`.
5. Actions tab > Deploy > Run workflow.

## Notes / next steps

- Database access uses pgx directly. Add [sqlc](https://sqlc.dev) later if you prefer generated queries.
- Set up automated Postgres backups (`pg_dump` to S3 or EBS snapshots) before real customers use it.
- Add rate limiting on `/auth/*` before going public.
- Next: admin endpoints to manage services, push notifications, LINE Login / PromptPay.
