# Lazy Database Backup Service Makefile

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary names
BINARY_NAME=lazy
EXAMPLE_BINARY=lazy-example

# Directories
BUILD_DIR=build
CMD_DIR=cmd
EXAMPLE_DIR=$(CMD_DIR)/example

# Version info
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.Commit=$(COMMIT)"

.PHONY: all build clean test coverage lint fmt vet deps tidy run-example help install-tools

# Default target
all: clean deps test build

# Build the main library (no main package, so we build the example)
build: deps
	@echo "Building example application..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY) ./$(EXAMPLE_DIR)
	@echo "Build complete: $(BUILD_DIR)/$(EXAMPLE_BINARY)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "Tests complete"

# Run tests with coverage report
coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint: install-tools
	@echo "Running linter..."
	$(GOLINT) run ./...
	@echo "Linting complete"

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .
	@echo "Formatting complete"

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...
	@echo "Vet complete"

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) verify
	@echo "Dependencies downloaded"

# Tidy dependencies
tidy:
	@echo "Tidying dependencies..."
	$(GOMOD) tidy
	@echo "Dependencies tidied"

# Run the example application
run-example: build
	@echo "Running example application..."
	./$(BUILD_DIR)/$(EXAMPLE_BINARY)

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin)
	@echo "Development tools installed"

# Build for multiple platforms
build-all: deps
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY)-linux-amd64 ./$(EXAMPLE_DIR)
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY)-linux-arm64 ./$(EXAMPLE_DIR)
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY)-darwin-amd64 ./$(EXAMPLE_DIR)
	
	# macOS ARM64 (Apple Silicon)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY)-darwin-arm64 ./$(EXAMPLE_DIR)
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(EXAMPLE_BINARY)-windows-amd64.exe ./$(EXAMPLE_DIR)
	
	@echo "Multi-platform build complete"

# Create release archives
release: build-all
	@echo "Creating release archives..."
	@mkdir -p $(BUILD_DIR)/releases
	
	# Create tar.gz for Unix systems
	@cd $(BUILD_DIR) && tar -czf releases/$(EXAMPLE_BINARY)-$(VERSION)-linux-amd64.tar.gz $(EXAMPLE_BINARY)-linux-amd64
	@cd $(BUILD_DIR) && tar -czf releases/$(EXAMPLE_BINARY)-$(VERSION)-linux-arm64.tar.gz $(EXAMPLE_BINARY)-linux-arm64
	@cd $(BUILD_DIR) && tar -czf releases/$(EXAMPLE_BINARY)-$(VERSION)-darwin-amd64.tar.gz $(EXAMPLE_BINARY)-darwin-amd64
	@cd $(BUILD_DIR) && tar -czf releases/$(EXAMPLE_BINARY)-$(VERSION)-darwin-arm64.tar.gz $(EXAMPLE_BINARY)-darwin-arm64
	
	# Create zip for Windows
	@cd $(BUILD_DIR) && zip releases/$(EXAMPLE_BINARY)-$(VERSION)-windows-amd64.zip $(EXAMPLE_BINARY)-windows-amd64.exe
	
	@echo "Release archives created in $(BUILD_DIR)/releases/"

# Run security scan
security:
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...
	@echo "Security scan complete"

# Generate documentation
docs:
	@echo "Generating documentation..."
	@which godoc > /dev/null || (echo "Installing godoc..." && go install golang.org/x/tools/cmd/godoc@latest)
	@echo "Starting documentation server at http://localhost:6060"
	@echo "Visit http://localhost:6060/pkg/github.com/vfa-khuongdv/lazy/ to view docs"
	godoc -http=:6060

# Database operations
db-migrate:
	@echo "Running database migrations..."
	@echo "Note: Migrations are handled automatically by the application"
	@echo "Ensure your database is running and accessible"

# Docker operations
docker-build:
	@echo "Building Docker image..."
	docker build -t lazy-backup:$(VERSION) .
	docker tag lazy-backup:$(VERSION) lazy-backup:latest
	@echo "Docker image built: lazy-backup:$(VERSION)"

docker-run:
	@echo "Running Docker container..."
	docker run --rm -it \
		-p 8081:8081 \
		-e MYSQL_HOST=host.docker.internal \
		-e MYSQL_PORT=3306 \
		-e MYSQL_USER=root \
		-e MYSQL_PASSWORD=root \
		-e MYSQL_DATABASE=golang_sync_db \
		lazy-backup:latest

# Development helpers
dev-setup: install-tools deps
	@echo "Setting up development environment..."
	@echo "Development environment ready!"

dev-test: fmt vet lint test
	@echo "Running full development test suite..."
	@echo "All checks passed!"

# Benchmark tests
benchmark:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
	@echo "Benchmarks complete"

# Check for outdated dependencies
deps-check:
	@echo "Checking for outdated dependencies..."
	$(GOCMD) list -u -m all
	@echo "Dependency check complete"

# Update dependencies
deps-update:
	@echo "Updating dependencies..."
	$(GOCMD) get -u ./...
	$(GOMOD) tidy
	@echo "Dependencies updated"

# Generate mocks (if using mockery)
mocks:
	@echo "Generating mocks..."
	@which mockery > /dev/null || (echo "Installing mockery..." && go install github.com/vektra/mockery/v2@latest)
	mockery --all --output=mocks --case=underscore
	@echo "Mocks generated"

# Help target
help:
	@echo "Available targets:"
	@echo "  build         - Build the example application"
	@echo "  build-all     - Build for multiple platforms"
	@echo "  clean         - Clean build artifacts"
	@echo "  test          - Run tests"
	@echo "  coverage      - Run tests with coverage report"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format code"
	@echo "  vet           - Run go vet"
	@echo "  deps          - Download dependencies"
	@echo "  tidy          - Tidy dependencies"
	@echo "  run-example   - Build and run example application"
	@echo "  install-tools - Install development tools"
	@echo "  release       - Create release archives"
	@echo "  security      - Run security scan"
	@echo "  docs          - Generate and serve documentation"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo "  dev-setup     - Set up development environment"
	@echo "  dev-test      - Run full development test suite"
	@echo "  benchmark     - Run benchmark tests"
	@echo "  deps-check    - Check for outdated dependencies"
	@echo "  deps-update   - Update dependencies"
	@echo "  mocks         - Generate mocks"
	@echo "  help          - Show this help message"