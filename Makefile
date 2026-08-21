# Variables
APP_NAME := dttmr-api
MAIN_DIR := ./cmd/api
BIN_DIR := ./bin

# Dynamically pull version and commit hash from Git
VERSION := $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse --short HEAD)
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Linker flags to strip symbols (-s -w) and inject build data into the binary
LDFLAGS := -w -s \
	-X 'main.Version=$(VERSION)' \
	-X 'main.Commit=$(COMMIT)' \
	-X 'main.BuildTime=$(BUILD_TIME)'

# Targets

.PHONY: all help build run test test-cover lint fmt mod clean docker

all: help

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'

## build: Compile the binary
build:
	@echo "Building $(APP_NAME) version $(VERSION)..."
	@CGO_ENABLED=0 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(APP_NAME) $(MAIN_DIR)

## run: Run the application directly
run: build
	@echo "Starting $(APP_NAME)..."
	@$(BIN_DIR)/$(APP_NAME)

## test: Run tests with race detector
test:
	@echo "Running tests..."
	@go test -v -race -timeout 30s ./...

## test-cover: Run tests and generate coverage report
test-cover:
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated at coverage.html"

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	@golangci-lint run ./...

## fmt: Format code and organize imports
fmt:
	@echo "Formatting code..."
	@go fmt ./...
	@go run golang.org/x/tools/cmd/goimports@latest -w .

## mod: Tidy and verify dependencies
mod:
	@echo "Tidying and verifying module dependencies..."
	@go mod tidy
	@go mod verify

## clean: Remove build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@go clean

## docker: Build a Docker image
docker:
	@echo "Building Docker image for $(APP_NAME):$(VERSION)..."
	@docker build -t $(APP_NAME):$(VERSION) -t $(APP_NAME):latest .

## swag: Create swagger documentation
swag:
	@echo "Creating swagger documentation for $(APP_NAME)"
	@swag init --outputTypes json,yaml -g $(MAIN_DIR)/main.go
