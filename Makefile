# StreamRecorder Makefile

.PHONY: test test-verbose test-coverage build clean help

# Default target
all: build

# Build the application
build:
	@echo "🔨 Building StreamRecorder..."
	go build -o stream-recorder.exe ./cmd/main

# Build for different platforms
build-windows:
	@echo "🔨 Building for Windows..."
	GOOS=windows GOARCH=amd64 go build -o stream-recorder-windows.exe ./cmd/main

build-linux:
	@echo "🔨 Building for Linux..."
	GOOS=linux GOARCH=amd64 go build -o stream-recorder-linux ./cmd/main

build-all: build-windows build-linux

# Run unit tests
test:
	@echo "🧪 Running unit tests..."
	go run test_runner.go

# Run tests with verbose output
test-verbose:
	@echo "🧪 Running unit tests (verbose)..."
	go test -v ./pkg/...

# Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -coverprofile=coverage.out ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "📊 Coverage report generated: coverage.html"

# Run tests for a specific package
test-pkg:
	@if [ -z "$(PKG)" ]; then \
		echo "❌ Please specify package: make test-pkg PKG=./pkg/config"; \
		exit 1; \
	fi
	@echo "🧪 Testing package: $(PKG)"
	go test -v $(PKG)

# Run benchmarks
benchmark:
	@echo "🏃 Running benchmarks..."
	go test -bench=. -benchmem ./pkg/...

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f stream-recorder.exe stream-recorder-windows.exe stream-recorder-linux
	rm -f coverage.out coverage.html
	rm -rf data/ out/ *.json

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# Lint code (requires golangci-lint)
lint:
	@echo "🔍 Linting code..."
	golangci-lint run

# Tidy dependencies
tidy:
	@echo "📦 Tidying dependencies..."
	go mod tidy

# Run security check (requires gosec)
security:
	@echo "🔒 Running security check..."
	gosec ./...

# Install development tools
install-tools:
	@echo "🛠️ Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Quick development cycle: format, tidy, build, test
dev: fmt tidy build test

# CI pipeline: format check, lint, security, test, build
ci: fmt tidy lint security test build

# Help
help:
	@echo "StreamRecorder Build Commands"
	@echo "============================="
	@echo ""
	@echo "Build Commands:"
	@echo "  build         - Build the main application"
	@echo "  build-windows - Build for Windows (x64)"
	@echo "  build-linux   - Build for Linux (x64)"
	@echo "  build-all     - Build for all platforms"
	@echo ""
	@echo "Test Commands:"
	@echo "  test          - Run unit tests with custom runner"
	@echo "  test-verbose  - Run tests with verbose output"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-pkg PKG=<pkg> - Test specific package"
	@echo "  benchmark     - Run benchmarks"
	@echo ""
	@echo "Quality Commands:"
	@echo "  fmt           - Format code"
	@echo "  lint          - Lint code (requires golangci-lint)"
	@echo "  security      - Security analysis (requires gosec)"
	@echo "  tidy          - Tidy dependencies"
	@echo ""
	@echo "Development Commands:"
	@echo "  dev           - Quick dev cycle (fmt, tidy, build, test)"
	@echo "  ci            - Full CI pipeline"
	@echo "  clean         - Clean build artifacts"
	@echo "  install-tools - Install development tools"
	@echo ""
	@echo "Examples:"
	@echo "  make test"
	@echo "  make test-pkg PKG=./pkg/config"
	@echo "  make build-all"
	@echo "  make dev"