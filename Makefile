.PHONY: all build test clean run install docker docker-build docker-run help

# Binary name
BINARY_NAME=s3dir

# Build variables
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)"

all: test build

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/s3dir
	@echo "Build complete: $(BINARY_NAME)"

## test: Run all tests
test:
	@echo "Running tests..."
	@go test -v -race -cover ./...

## test-coverage: Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## bench: Run benchmarks
bench:
	@echo "Running benchmarks..."
	@go test -bench=. -benchmem ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -f *.prof
	@echo "Clean complete"

## run: Run the application
run: build
	@./$(BINARY_NAME)

## run-dev: Run with development settings
run-dev:
	@S3DIR_VERBOSE=true S3DIR_PORT=8000 go run ./cmd/s3dir

## install: Install the binary
install:
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) ./cmd/s3dir
	@echo "Install complete"

## lint: Run linter
lint:
	@echo "Running linter..."
	@golangci-lint run

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@gofmt -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@go vet ./...

## docker-build: Build Docker image
docker-build:
	@echo "Building Docker image..."
	@docker build -t s3dir:$(VERSION) -t s3dir:latest .
	@echo "Docker build complete"

## docker-run: Run Docker container
docker-run:
	@echo "Running Docker container..."
	@docker run -d -p 8000:8000 -v $(PWD)/data:/data --name s3dir s3dir:latest

## docker-stop: Stop Docker container
docker-stop:
	@docker stop s3dir
	@docker rm s3dir

## docker-compose-up: Start with docker-compose
docker-compose-up:
	@docker-compose up -d

## docker-compose-down: Stop docker-compose
docker-compose-down:
	@docker-compose down

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@go mod download
	@go mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
