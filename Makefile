# AIMem Intelligent Session Management System
# Development and Testing Makefile

.PHONY: help build test test-unit test-integration test-e2e test-performance test-all clean lint format deps check install run-dev benchmark coverage docs migrate

# Default target
help:
	@echo "🧠 AIMem Development Commands"
	@echo "============================"
	@echo ""
	@echo "📦 Build Commands:"
	@echo "  build        - Build the AIMem binary"
	@echo "  install      - Install AIMem globally"
	@echo "  clean        - Clean build artifacts"
	@echo ""
	@echo "🧪 Test Commands:"
	@echo "  test         - Run all tests with custom test runner"
	@echo "  test-unit    - Run unit tests only"
	@echo "  test-integration - Run integration tests"
	@echo "  test-e2e     - Run end-to-end tests"
	@echo "  test-performance - Run performance tests"
	@echo "  test-all     - Run all test suites with Go"
	@echo "  coverage     - Generate test coverage report"
	@echo "  benchmark    - Run benchmarks"
	@echo ""
	@echo "🔧 Development Commands:"
	@echo "  run-dev      - Run in development mode"
	@echo "  lint         - Run linters"
	@echo "  format       - Format code"
	@echo "  deps         - Download dependencies"
	@echo "  check        - Run all checks (lint + test)"
	@echo ""
	@echo "📚 Documentation:"
	@echo "  docs         - Generate documentation"
	@echo ""
	@echo "🔄 Database Commands:"
	@echo "  migrate      - Run database migrations"
	@echo ""

# Build commands
build:
	@echo "🔨 Building AIMem..."
	@go build -ldflags "-X main.version=$$(git describe --tags --always --dirty)" -o bin/aimem ./cmd/aimem

install: build
	@echo "📦 Installing AIMem..."
	@go install ./cmd/aimem

clean:
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf bin/
	@rm -rf tmp/
	@rm -rf coverage.out
	@rm -rf coverage.html
	@go clean -cache

# Test commands
test:
	@echo "🧪 Running tests with custom runner..."
	@go run tests/run_tests.go

test-unit:
	@echo "🔬 Running unit tests..."
	@go test -v -race -timeout 5m ./internal/...

test-integration:
	@echo "🔗 Running integration tests..."
	@go test -v -race -timeout 10m ./tests/integration/...

test-e2e:
	@echo "🌐 Running end-to-end tests..."
	@go test -v -race -timeout 15m ./tests/e2e/...

test-performance:
	@echo "⚡ Running performance tests..."
	@go test -v -bench=. -benchtime=5s -timeout 20m ./tests/performance/...

test-all:
	@echo "🎯 Running all tests..."
	@go test -v -race -timeout 30m ./...

coverage:
	@echo "📊 Generating test coverage..."
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

benchmark:
	@echo "🏃‍♂️ Running benchmarks..."
	@go test -bench=. -benchmem -benchtime=10s ./...

# Development commands
run-dev:
	@echo "🚀 Running AIMem in development mode..."
	@go run ./cmd/aimem --config-file dev-config.yaml --log-level debug

lint:
	@echo "🔍 Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout 5m; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
		go vet ./...; \
	fi

format:
	@echo "✨ Formatting code..."
	@go fmt ./...
	@if command -v goimports >/dev/null 2>&1; then \
		goimports -w .; \
	else \
		echo "⚠️  goimports not installed. Install with: go install golang.org/x/tools/cmd/goimports@latest"; \
	fi

deps:
	@echo "📥 Downloading dependencies..."
	@go mod download
	@go mod tidy

check: lint test-unit
	@echo "✅ All checks passed!"

# Documentation
docs:
	@echo "📖 Generating documentation..."
	@if command -v godoc >/dev/null 2>&1; then \
		echo "Starting godoc server at http://localhost:6060"; \
		godoc -http=:6060; \
	else \
		echo "⚠️  godoc not installed. Install with: go install golang.org/x/tools/cmd/godoc@latest"; \
	fi

# Database commands
migrate:
	@echo "🔄 Running database migrations..."
	@go run ./cmd/aimem migrate --config-file config.yaml

# Development setup
setup-dev:
	@echo "🛠️  Setting up development environment..."
	@go mod download
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	@if ! command -v goimports >/dev/null 2>&1; then \
		echo "Installing goimports..."; \
		go install golang.org/x/tools/cmd/goimports@latest; \
	fi
	@if ! command -v godoc >/dev/null 2>&1; then \
		echo "Installing godoc..."; \
		go install golang.org/x/tools/cmd/godoc@latest; \
	fi
	@echo "✅ Development environment ready!"

# Quick commands for common workflows
quick-test: test-unit
	@echo "⚡ Quick test completed!"

quick-check: format lint test-unit
	@echo "⚡ Quick check completed!"

full-check: format lint test-all coverage
	@echo "🎯 Full check completed!"

# CI/CD commands
ci-test:
	@echo "🤖 Running CI tests..."
	@go test -race -coverprofile=coverage.out -timeout 30m ./...
	@go tool cover -func=coverage.out

ci-build:
	@echo "🤖 Running CI build..."
	@go build -ldflags "-X main.version=$$(git describe --tags --always --dirty)" ./cmd/aimem

# Performance monitoring
profile-cpu:
	@echo "📈 Profiling CPU usage..."
	@go test -cpuprofile cpu.prof -bench . ./tests/performance/...
	@go tool pprof cpu.prof

profile-memory:
	@echo "🧠 Profiling memory usage..."
	@go test -memprofile mem.prof -bench . ./tests/performance/...
	@go tool pprof mem.prof

# Release commands
release-dry-run:
	@echo "🚀 Dry run release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --snapshot --rm-dist; \
	else \
		echo "⚠️  goreleaser not installed. Install from: https://goreleaser.com/install/"; \
	fi

release:
	@echo "🚀 Creating release..."
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser release --rm-dist; \
	else \
		echo "⚠️  goreleaser not installed. Install from: https://goreleaser.com/install/"; \
	fi

# Docker commands
docker-build:
	@echo "🐳 Building Docker image..."
	@docker build -t aimem:latest .

docker-test:
	@echo "🐳 Running tests in Docker..."
	@docker run --rm aimem:latest make test

# Database utilities
db-reset:
	@echo "🗃️  Resetting database..."
	@rm -f aimem.db
	@go run ./cmd/aimem migrate --config-file config.yaml

db-backup:
	@echo "💾 Backing up database..."
	@cp aimem.db aimem.db.backup.$$(date +%Y%m%d_%H%M%S)
	@echo "Backup created: aimem.db.backup.$$(date +%Y%m%d_%H%M%S)"

# Development utilities
watch:
	@echo "👁️  Watching for changes..."
	@if command -v entr >/dev/null 2>&1; then \
		find . -name '*.go' | entr -r make quick-test; \
	else \
		echo "⚠️  entr not installed. Install with package manager"; \
		echo "On macOS: brew install entr"; \
		echo "On Ubuntu: apt-get install entr"; \
	fi

serve-coverage:
	@echo "🌐 Serving coverage report..."
	@if [ -f coverage.html ]; then \
		python3 -m http.server 8080 --directory . & \
		echo "Coverage report available at: http://localhost:8080/coverage.html"; \
		echo "Press Ctrl+C to stop server"; \
		wait; \
	else \
		echo "⚠️  No coverage report found. Run 'make coverage' first"; \
	fi

# Git hooks
install-git-hooks:
	@echo "🪝 Installing git hooks..."
	@echo '#!/bin/sh\nmake quick-check' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ Pre-commit hook installed!"

# Version info
version:
	@echo "📋 AIMem Version Information"
	@echo "=========================="
	@echo "Git Version: $$(git describe --tags --always --dirty)"
	@echo "Go Version: $$(go version)"
	@echo "Git Branch: $$(git branch --show-current)"
	@echo "Git Commit: $$(git rev-parse HEAD)"
	@echo "Build Time: $$(date -u +%Y-%m-%dT%H:%M:%SZ)"