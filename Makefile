.PHONY: all api ui dev install build test clean help seed

# Default target
all: help

# Run the API server
api:
	go run cmd/server/main.go

# Run the UI (Vite)
ui:
	cd web && npm run dev

# Run both API and UI concurrently (fresh DB each time)
dev:
	rm -f openchore.db openchore.db-shm openchore.db-wal
	go run seed.go
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

# Seed the database
seed:
	go run seed.go

# Show help
help:
	@echo "Available targets:"
	@echo "  api     - Run the API server (Go)"
	@echo "  ui      - Run the UI (Vite)"
	@echo "  dev     - Run both API and UI concurrently"
	@echo "  install - Install dependencies for both API and UI"
	@echo "  build   - Build both API and UI"
	@echo "  test    - Run Go tests"
	@echo "  clean   - Clean up build artifacts and database"
	@echo "  seed    - Seed the database with initial data"
