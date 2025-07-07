# Variables
IMAGE_NAME=zoom
CONTAINER_NAME=zoom_container
PORT=8080
OUT_DIR := ./bin

# Targets
.DEFAULT_GOAL:=help
.PHONY: build help

all: build ## Run test, then build

build: ## Build the binary
	go build -o $(OUT_DIR)/main ./src/main

help: ## Display this help
    @grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'