# This make-sanity boilerplate courtesy of https://tech.davis-hansson.com/p/make/
.RECIPEPREFIX = >
SHELL := bash
.ONESHELL:
.SHELLFLAGS := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS += --warn-undefined-variables
MAKEFLAGS += --no-builtin-rules

# You can set env variables consistently here.
ifneq ($(strip $(wildcard .env)),)
include .env
export $(shell sed 's/=.*//' .env)
endif

BINARIES := sip-capture

GITREF=$(strip $(shell [ -d .git ] && git rev-parse --short HEAD))
VERSION=$(strip $(shell [ -d .git ] && git describe --always --tags --dirty))
BRANCH=$(strip $(shell [ -d .git ] && git rev-parse --abbrev-ref HEAD))
DATE=$(shell date -u -Iseconds)
export GITREF VERSION DATE BRANCH

# setup: > go mod download; install golangci-lint
# lint:
# cover:
# ci: lint test

build: ${BINARIES}  ## build all the binaries.
.PHONY: build

clean:  ## remove any previously built binary.
> @rm -f ${BINARIES}
.PHONY: clean

test:  ## run tests
> go test -cover ./...
.PHONY: test

lint:  ## run linting
> golangci-lint run

sip-capture: $(shell find . -type f -name '*.go')  ## build the sip-capture binary
> #CGO_LDFLAGS+="-L/usr/lib/x86_64-linux-gnu/libpcap.a" go build -ldflags="-s -w -linkmode external -extldflags \"-static\"" .
> go build -ldflags "-s -w -X=main.Version=$${VERSION} -X=main.Build=$${GITREF} -X=main.Branch=$${BRANCH} -X=main.Date=$${DATE}" -o sip-capture .

docker: $(shell find . -type f -name '*.go')  ## build a docker image.
> IMAGE_TAG=$${CODEBUILD_GIT_SHORT_COMMIT:=latest}
> docker build --build-arg VERSION=${VERSION} --build-arg BUILD_REF=${GITREF} --build-arg BUILD_DATE=${DATE} -t sip-capture:$${VERSION} .
.PHONY: docker

up: ## start a docker environment with MQTT and testing apps
> docker-compose build
> docker-compose up -d
.PHONY: up

local-run: sip-capture  ## Run a local copy not in docker (requires sudo)
> exec sudo ./sip-capture -topic /test/sip-data -log-level debug -sip-filter "(all (methods invite) request)"
.PHONY: local-run

# http://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
help:
> @grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
.PHONY: help

.DEFAULT_GOAL := build
