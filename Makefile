.PHONY: all build build-linux generate dev frontend frontend-dev mailpit clean

# Git commit baked into release binaries (shown in the UI and /api/me).
COMMIT ?= $(shell git describe --always --dirty --abbrev=8 2>/dev/null || echo unknown)
LDFLAGS = -X main.commit=$(COMMIT)

all: generate frontend build

# Go backend (production - with embedded frontend)
build: frontend
	rm -rf cmd/server/frontend_dist
	cp -r frontend/dist cmd/server/frontend_dist
	go build -ldflags "$(LDFLAGS)" -o filebox ./cmd/server

# Go backend (production - linux amd64)
build-linux: frontend
	rm -rf cmd/server/frontend_dist
	cp -r frontend/dist cmd/server/frontend_dist
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o filebox-linux-amd64 ./cmd/server

# Go backend (development - no embedded frontend)
dev:
	go run -tags dev ./cmd/server

# SQLc code generation
generate:
	sqlc generate

# Frontend
frontend:
	cd frontend && pnpm install && pnpm run build

frontend-dev:
	cd frontend && pnpm run dev

# Local SMTP catch-all for development: SMTP on :1025, web UI on :8025.
# Nothing it receives ever leaves the machine.
mailpit:
	docker run --rm -p 1025:1025 -p 8025:8025 axllent/mailpit:v1.30.7

# Clean build artifacts
clean:
	rm -f filebox filebox-linux-amd64
	rm -rf frontend/dist
	rm -rf cmd/server/frontend_dist
