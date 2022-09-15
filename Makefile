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

all: build

build: $(SRC)
	@$(GO) build $(GO_FLAGS) -o ${BINARY} $(SRC)

test:
	@go test -v ./...

docker_image_build:
	@docker-compose build

docker_image_push: docker_image_build
	@docker-compose push

clean:
	@$(GO) clean ./...
	@rm -f $(BINARY)

.PHONY: install test clean  docker_image_build docker_image_push
