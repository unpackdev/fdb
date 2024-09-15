.DEFAULT_GOAL := help

BIN_NAME := build/fdb
PKG := fdb
VERSION := 0.1.0
UNAME_S := $(shell uname -s 2>/dev/null || echo "unknown")
UNAME_S_LOWERCASE := $(shell echo $(UNAME_S) | tr A-Z a-z)
BUILD_TARGET := build-$(UNAME_S_LOWERCASE)
COMMIT_HASH := $(shell git rev-parse HEAD)

.PHONY: submodule
submodule: ## Update submodules
	git submodule update --init --recursive

.PHONY: help
help: ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: deps
deps: ## Install dependencies
ifeq ($(UNAME_S),Linux)
	sudo apt-get update && sudo apt-get install -y golang sqlite3 redis-server pipx
	sudo -v ; curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sudo sh -s -- -b $(go env GOPATH)/bin v1.58.1
	sudo -v ; curl https://rclone.org/install.sh | sudo bash
	pipx ensurepath
	pipx install slither-analyzer
endif
ifeq ($(UNAME_S),Darwin)
	brew install go sqlite golangci-lint redis pipx
	sudo -v ; curl https://rclone.org/install.sh | sudo bash
	pipx ensurepath
	pipx install slither-analyzer
endif
ifeq ($(OS),Windows_NT)
	choco install golang sqlite golangci-lint redis
endif

.PHONY: lint
lint: ## Lint the Go code using golangci-lint
	golangci-lint run

.PHONY: build
build: build-linux ## Build the binary for the current OS/Arch

.PHONY: build-linux
build-linux: ## Build the binary for Linux
	GOOS=linux GOARCH=amd64 go build -o ./$(BIN_NAME) -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH)" ./entrypoint/main.go

.PHONY: build-darwin
build-darwin: ## Build the binary for MacOS
	GOOS=darwin GOARCH=amd64 go build -o ./$(BIN_NAME) -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH)" ./entrypoint/main.go

.PHONY: build-windows
build-windows: ## Build the binary for Windows
	GOOS=windows GOARCH=amd64 go build -o ./$(BIN_NAME).exe -ldflags "-X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH)" ./entrypoint/main.go

.PHONY: run
run: build ## Run the binary
	./$(BIN_NAME)

.PHONY: test
test: ## Run tests
	go test -v -cover $(go list ./... | grep -v '^./libs')

.PHONY: benchmark
benchmark: ## Run benchmarks
	go test -v -bench . -benchmem ./... > benchmark.txt

.PHONY: submodules
submodules: ## Update submodules
	git submodule update --init --recursive

.PHONY: load-env
load-env:
	export $(grep -v '^#' .env | xargs)

.PHONY: clean
clean: ## Clean build files
ifeq ($(OS),Windows_NT) # Windows
	del /Q $(BIN_NAME).exe
else
	rm -f $(BIN_NAME)
endif