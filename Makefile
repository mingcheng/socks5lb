#https://gqlxj1987.github.io/2018/08/14/good-makefile-golang/
-include .env

PROJECT=socks5lb
VERSION ?= 1.4.0
COMMIT_HASH=$(shell git rev-parse --short HEAD)
SRC=./cmd/$(PROJECT)
BINARY=$(PROJECT)
GO ?= go
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
	@$(GO) test ./...

race:
	@CGO_ENABLED=1 $(GO) test -race ./...

vet:
	@$(GO) vet ./...

fmt:
	@gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)" || exit $$?; \
	if [ -n "$$files" ]; then \
		printf 'Run gofmt on:\n%s\n' "$$files"; \
		exit 1; \
	fi

check: fmt-check vet test race

docker_image_build:
	@docker-compose build

docker_image_push: docker_image_build
	@docker-compose push

clean:
	@$(GO) clean ./...
	@rm -f $(BINARY)

.PHONY: all build test race vet fmt fmt-check check clean docker_image_build docker_image_push
