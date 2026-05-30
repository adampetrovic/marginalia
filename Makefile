# Marginalia build orchestration.
#
# The web UI is a Vite/React app in frontend/. It is built to static assets and
# committed into service/internal/web/dist/, which the Go binary embeds. Run
# `make frontend` after changing anything under frontend/ and commit the result.

.PHONY: all frontend build build-go test test-go lint run clean

# Full build from clean: frontend assets + single binary.
all: frontend build-go

# Build the React frontend and sync its output into the Go embed directory.
frontend:
	cd frontend && npm ci && npm run build
	rm -rf service/internal/web/dist
	cp -r frontend/dist service/internal/web/dist

# Build the single binary, rebuilding the frontend first.
build: frontend build-go

# Build only the Go binary (assumes service/internal/web/dist is up to date).
build-go:
	cd service && CGO_ENABLED=1 go build -o ../marginalia ./cmd/marginalia

# Run the full test suite.
test: test-go

test-go:
	cd service && go test ./...

lint:
	cd service && go vet ./...

# Run the server (serves the embedded UI at http://localhost:8080).
run:
	cd service && go run ./cmd/marginalia

clean:
	rm -f marginalia
	rm -rf frontend/dist
