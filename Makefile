.PHONY: build dev test test-all clean run

# Build the production binary with embedded frontend
build: frontend-build
	go build -o warpspawn ./cmd/warpspawn

# Development mode - Go only (no frontend embedding)
dev:
	go run ./cmd/warpspawn --debug

# Run tests
test:
	go test ./... -v

# Run tests with coverage
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Frontend build
frontend-build:
	cd frontend && npm run build 2>/dev/null || true

# Clean build artifacts
clean:
	rm -f warpspawn coverage.out coverage.html
	rm -rf frontend/dist

# Run the built binary
run: build
	./warpspawn
