.PHONY: all api ui dev install build test clean help

# Default target
all: help

# Run the API server
api:
	go run cmd/server/main.go

# Run the UI (Vite)
ui:
	cd web && npm run dev

# Run both API and UI concurrently (fresh DB each time)
# The server auto-seeds from config/config.yaml on first boot when the DB is empty.
dev:
	@test -f config/config.yaml || (cp config/config.example.yaml config/config.yaml && echo "Created config/config.yaml from example")
	rm -f openchore.db openchore.db-shm openchore.db-wal
	make -j 2 api ui

# Install dependencies for both
install:
	go mod download
	cd web && npm install

# Build both API and UI
build:
	go build -o server cmd/server/main.go
	cd web && npm run build

# Run Go tests
test:
	go test ./...

# Clean up build artifacts and database
clean:
	rm -f server openchore.db openchore.db-shm openchore.db-wal

# Show help
help:
	@echo "Available targets:"
	@echo "  api     - Run the API server (Go)"
	@echo "  ui      - Run the UI (Vite)"
	@echo "  dev     - Run both API and UI concurrently (fresh DB, auto-seeded from config)"
	@echo "  install - Install dependencies for both API and UI"
	@echo "  build   - Build both API and UI"
	@echo "  test    - Run Go tests"
	@echo "  clean   - Clean up build artifacts and database"
