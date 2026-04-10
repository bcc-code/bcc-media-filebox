.PHONY: all build build-linux generate dev frontend frontend-dev clean

all: generate frontend build

# Go backend (production - with embedded frontend)
build: frontend
	rm -rf cmd/server/frontend_dist
	cp -r frontend/dist cmd/server/frontend_dist
	go build -o file-pusher ./cmd/server

# Go backend (production - linux amd64)
build-linux: frontend
	rm -rf cmd/server/frontend_dist
	cp -r frontend/dist cmd/server/frontend_dist
	GOOS=linux GOARCH=amd64 go build -o file-pusher-linux-amd64 ./cmd/server

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

# Clean build artifacts
clean:
	rm -f file-pusher file-pusher-linux-amd64
	rm -rf frontend/dist
	rm -rf cmd/server/frontend_dist
