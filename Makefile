#https://gqlxj1987.github.io/2018/08/14/good-makefile-golang/
include .env

PROJECT=$(shell basename "$(PWD)")
VERSION=$(shell date +%Y%m%d)
COMMIT_HASH=$(shell git rev-parse --short HEAD)

SRC=./cmd/$(PROJECT)
BINARY=$(PROJECT)

GO_FLAGS=-ldflags="\
	-X github.com/mingcheng/socks5lb.Version=$(VERSION) \
	-X 'github.com/mingcheng/socks5lb.BuildCommit=$(COMMIT_HASH)' \
	-X 'github.com/mingcheng/socks5lb.BuildDate=$(shell date)'"

GO=$(shell which go)

PACKAGES=`go list ./...`
GOFILES=`find . -name "*.go" -type f`

# Make is verbose in Linux. Make it silent.
MAKEFLAGS += --silent

all: build

build: cmd/$(PROJECT)
	@$(GO) build $(GO_FLAGS) -o ${BINARY} $(SRC)

list:
	@echo ${PACKAGES}
	@echo ${VETPACKAGES}
	@echo ${GOFILES}

test:
	@go test -v ./...

docker_image_build:
	@docker-compose build

docker_image_push: docker_image_build
	@docker-compose push

clean:
	@$(GO) clean ./...
	@rm ${BINARY}

.PHONY: install test clean target docker_image_build docker_image_push
