# Makefile for Ping Probe application

# Application name
APP_NAME = cw-ping-probe

# Go parameters
GOCMD = go
GOBUILD = $(GOCMD) build
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod

# Build directory
BUILD_DIR = build

# Version information to embed in the binary
VERSION = 1.0.0
BUILD_TIME = $(shell date -u '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT = $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LD_FLAGS = -ldflags="-s -w -X 'main.Version=$(VERSION)' -X 'main.BuildTime=$(BUILD_TIME)' -X 'main.GitCommit=$(GIT_COMMIT)'"

# List of targets
.PHONY: all build clean test deps mac linux windows arm64 amd64

# Default target
all: clean buildall

# Build all targets
buildall: mac-amd64 mac-arm64 linux-amd64 linux-arm64 windows-amd64

# Create build directory if it doesn't exist
$(BUILD_DIR):
	mkdir -p $(BUILD_DIR)

# Clean build directory
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) tidy
	$(GOGET) -v

# macOS AMD64 build
mac-amd64: $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-amd64 .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)-darwin-amd64"

# macOS ARM64 build (Apple Silicon)
mac-arm64: $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-darwin-arm64 .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)-darwin-arm64"

# Linux AMD64 build
linux-amd64: $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-amd64 .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)-linux-amd64"

# Linux ARM64 build
linux-arm64: $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-linux-arm64 .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)-linux-arm64"

# Windows AMD64 build
windows-amd64: $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LD_FLAGS) -o $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe .
	@echo "Build completed: $(BUILD_DIR)/$(APP_NAME)-windows-amd64.exe"

# Convenient targets for specific platforms/architectures
mac: mac-amd64 mac-arm64
linux: linux-amd64 linux-arm64
windows: windows-amd64

# Architecture-specific targets
amd64: mac-amd64 linux-amd64 windows-amd64
arm64: mac-arm64 linux-arm64

# Release with zipped binaries
release: build
	@echo "Creating release archives..."
	cd $(BUILD_DIR) && tar -czvf $(APP_NAME)-darwin-amd64.tar.gz $(APP_NAME)-darwin-amd64
	cd $(BUILD_DIR) && tar -czvf $(APP_NAME)-darwin-arm64.tar.gz $(APP_NAME)-darwin-arm64
	cd $(BUILD_DIR) && tar -czvf $(APP_NAME)-linux-amd64.tar.gz $(APP_NAME)-linux-amd64
	cd $(BUILD_DIR) && tar -czvf $(APP_NAME)-linux-arm64.tar.gz $(APP_NAME)-linux-arm64
	cd $(BUILD_DIR) && zip $(APP_NAME)-windows-amd64.zip $(APP_NAME)-windows-amd64.exe
	@echo "Release archives created in $(BUILD_DIR)"

# Run the application (for development)
run:
	$(GOCMD) run .