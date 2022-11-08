#https://gqlxj1987.github.io/2018/08/14/good-makefile-golang/
include .env

PROJECT=socks5lb
VERSION=$(shell date +%Y%m%d)
COMMIT_HASH=$(shell git rev-parse --short HEAD)
SRC=./cmd/$(PROJECT)
BINARY=$(PROJECT)
GO=$(shell which go)
GO_FLAGS=-ldflags="\
	-X 'github.com/mingcheng/socks5lb.Version=$(VERSION)' \
	-X 'github.com/mingcheng/socks5lb.BuildCommit=$(COMMIT_HASH)' \
	-X 'github.com/mingcheng/socks5lb.BuildDate=$(shell date)'"

# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

all: clean build

build: $(SRC)
	@$(GO) build $(GO_FLAGS) -o ${BINARY} $(SRC)

# https://dev.to/thewraven/universal-macos-binaries-with-go-1-16-3mm3
universal:
	GOOS=darwin GOARCH=amd64 $(GO) build $(GO_FLAGS) -o ${BINARY}_amd64 $(SRC)
	GOOS=darwin GOARCH=arm64 $(GO) build $(GO_FLAGS) -o ${BINARY}_arm64 $(SRC)
	@lipo -create -output ${BINARY} ${BINARY}_amd64 ${BINARY}_arm64
	@rm -f ${BINARY}_amd64 ${BINARY}_arm64

test:
	@go test -v ./...

docker_image_build:
	@docker-compose build

docker_image_push: docker_image_build
	@docker-compose push

clean:
	@$(GO) clean ./...
	@rm -f $(BINARY)

.PHONY: install test clean universal docker_image_build docker_image_push
