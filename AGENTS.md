# AGENTS.md

## Quick Start

```bash
./build_dev.sh dev      # deps -> docs -> build -> run
./build_dev.sh test     # go test ./... -v
./build_dev.sh lint     # golangci-lint run
./build_dev.sh all      # full CI: deps, docs, lint, test, build
./build_dev.sh docs     # swag init -g main.go -o docs/
./build_dev.sh docker   # docker build
./build_dev.sh clean    # rm binary + docs/
```

## Runtime

- This service uses **Dapr** as the microservices runtime. Production command:
  ```bash
  dapr run --app-id box-manage-service --app-port 8080 -- go run main.go
  ```
- `main.go` calls `daprd.NewServiceWithMux` to create the server — the Chi router is handed to Dapr, not to `net/http` directly.

## Config

- All config via **environment variables**. See `config/config.go` for the full list.
- `.env` is loaded by `godotenv` on startup for local dev.
- Key env: `BASE_CONTEXT` sets a path prefix for all routes (e.g., `/api/box-manage-service`).

## Architecture

```
api/controllers -> service -> repository -> GORM -> PostgreSQL
```

- **Controllers**: HTTP handlers (Chi router, `go-chi/render` for responses)
- **Services**: Business logic; interfaces defined in `service/interfaces.go`
- **Repositories**: Data access; generic `BaseRepository[T]` + domain repos in `repository/interfaces.go`
- **Models**: GORM entities in `models/`

## Migration & Codegen

- Auto-migration runs on startup (configurable). It migrates all GORM models, executes pre-migration patches, and runs embedded SQL files.
- SQL files in `migrations/` are embedded at compile time via Go `embed` (`migrations/embed.go`).
- **Swagger docs must be regenerated** after API changes: `./build_dev.sh docs` (requires `swag` CLI).
- Swagger is served at `{BASE_CONTEXT}/swagger/index.html`.

## Testing

```bash
go test ./... -v
go test ./service/ -run TestAutoSchedulerService_StartStop -v   # single test
go test ./models/ -run TestSchedulePolicy -v                    # single test
```

Only 2 test files exist: `service/auto_scheduler_service_test.go` and `models/schedule_policy_test.go`. Tests use `stretchr/testify` (assert + mock).

## Linting

`golangci-lint run` — no `.golangci.yml` config file in the repo; relies on defaults.

## Docker

Multi-stage build (`golang:1.23.1-alpine` -> `alpine:3.19`). Includes `ffmpeg`, `ca-certificates`, `tzdata` in the runtime image.
Data dirs in container: `/app/data/video`, `/app/data/models`, `/app/temp`.

## CI

Push/PR to `main` triggers `.github/workflows/docker-image.yml`: builds multi-arch image (amd64, arm64) and pushes to Aliyun ACR.

## Notable

- SSE (Server-Sent Events) is used for real-time updates (10+ event channels defined in `api/routes.go`).
- The `client/` directory contains HTTP clients that proxy requests to box devices and ZLMediaKit.
- `service/executors/` implements a workflow orchestration engine with node types: HTTP, Python, MQTT, KVM, reasoning, etc.
- `go-chi/cors` middleware is configured from env vars (not hardcoded).
