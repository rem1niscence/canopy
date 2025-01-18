# Variables
GO_BIN_DIR := ~/go/bin
CLI_DIR := ./cmd/cli/...
WALLET_DIR := ./cmd/rpc/web/wallet
EXPLORER_DIR := ./cmd/rpc/web/explorer
DOCKER_DIR := ./.docker

# Targets
.PHONY: build deps test build-wallet build-explorer docker-up docker-up-fast

build:
	go build -o $(GO_BIN_DIR)/canopy $(CLI_DIR)

deps:
	go mod vendor

test:
	go test ./... -p=1

build-wallet:
	cd $(WALLET_DIR) && npm install && npm run build

build-explorer:
	cd $(EXPLORER_DIR) && npm install && npm run build

docker-up:
	cd $(DOCKER_DIR) && docker-compose down && docker-compose up --build

docker-down:
	cd $(DOCKER_DIR) && docker-compose down

docker-up-fast:
	cd $(DOCKER_DIR) && docker-compose down && docker-compose up
