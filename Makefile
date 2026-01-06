.PHONY: help build run test clean docker-build docker-run docker-stop install-deps

help:
	@echo "Available targets:"
	@echo "  make build          - Build the server binary"
	@echo "  make run            - Run the server"
	@echo "  make test           - Run tests"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run with Docker Compose"
	@echo "  make docker-stop    - Stop Docker containers"
	@echo "  make install-deps   - Install Go dependencies"

build:
	@echo "Building server..."
	CGO_ENABLED=1 go build -o news-server ./cmd/server
	@echo "Build complete: ./news-server"

run:
	@echo "Starting server..."
	go run ./cmd/server

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning build artifacts..."
	rm -f news-server
	rm -rf data/*.db data/*.bleve
	@echo "Clean complete"

docker-build:
	@echo "Building Docker image..."
	docker-compose build
	@echo "Docker build complete"

docker-run:
	@echo "Starting services with Docker Compose..."
	docker-compose up -d
	@echo "Services started. Check logs with: docker-compose logs -f"

docker-stop:
	@echo "Stopping Docker services..."
	docker-compose down
	@echo "Services stopped"

install-deps:
	@echo "Installing Go dependencies..."
	go mod download
	go mod tidy
	@echo "Dependencies installed"

.DEFAULT_GOAL := help
