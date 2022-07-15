PROJECT=socks5lb
VERSION=`date +%Y%m%d`
COMMIT_HASH=`git rev-parse --short HEAD`

SRC=./cmd/$(PROJECT)
BINARY=$(PROJECT)

GO_ENV=CGO_ENABLED=0
GO_FLAGS=-ldflags="-X main.version=$(VERSION) -X 'main.commit=$(COMMIT_HASH)' -X 'main.date=`date`'"
GO=go

PACKAGES=`go list ./...`
GOFILES=`find . -name "*.go" -type f`

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
