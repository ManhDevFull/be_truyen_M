# Backend (Go/Fiber)

## Prerequisites
- Go 1.22+
- PostgreSQL 16+
- Redis 7+

## Config
Copy env file and edit:
- `cp .env.example .env`

Environment variables:
- `APP_ENV` (dev|prod)
- `APP_PORT` (default `:8080`)
- `JWT_SECRET`
- `CORS_ORIGINS` (comma-separated origins, default `http://localhost:3000,http://localhost:3001`)
- `DB_HOST`, `DB_PORT` (default 5433), `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`
- `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`

## Run Local
From `backend/`:
- Install deps: `go mod download`
- Start API: `go run ./cmd/api`

Local defaults assume Postgres on port 5433 with user `manh` and Redis with password `manh123`.

Redis local (Docker):
- `docker run -d --name redis-local -p 6379:6379 redis:7 redis-server --requirepass manh123`

Health check:
- `curl http://localhost:8080/healthz`

## Test Local
From `backend/`:
- Run tests: `go test ./...`

## Build
From `backend/`:
- `go build -o bin/api ./cmd/api`

## Deploy (Docker)
From repo root:
- `docker compose up -d --build go-api`

## API Notes
Public endpoints:
- `POST /api/auth/register`
- `POST /api/auth/login` (email + password only)
- `POST /api/auth/refresh` (refresh via httpOnly cookie)
- `POST /api/auth/logout`
- `GET /api/comics`
- `GET /api/comics/:id`
- `GET /api/chapter/:id`

Auth endpoints:
- `POST /api/ads/check`
- `POST /api/reward/read`

Admin endpoints (require admin role):
- `GET /api/admin/stats`
- `GET /api/admin/users`
- `POST /api/admin/user/ban`
- `POST /api/admin/user/reset-points`
- `POST /api/admin/comic`
- `PUT /api/admin/comic/:id`
- `DELETE /api/admin/comic/:id`
- `GET /api/admin/analytics` (not implemented)
- `POST /api/admin/reward/distribute` (not implemented)

Auth notes:
- Access token TTL: 5 minutes
- Refresh token stored in httpOnly cookie and rotated on refresh

Admin roles:
- `owner` (full access)
- `admin` (full access, below owner)
- `staff` (limited admin permissions)
