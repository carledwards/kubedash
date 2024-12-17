NAME        := kubism
GO_FLAGS    ?=
GO_TAGS     ?= netgo
CGO_ENABLED?=0
OUTPUT_BIN  ?= execs/$(NAME)
PACKAGE     := github.com/carledwards/$(NAME)
GIT_REV     ?= $(shell git rev-parse --short HEAD)
SOURCE_DATE_EPOCH ?= $(shell date +%s)
DATE        ?= $(shell TZ=UTC date -j -f "%s" ${SOURCE_DATE_EPOCH} +"%Y-%m-%d:%H:%M:%SZ")

default: help

test: ## Run all tests
	@go clean --testcache && go test ./...

cover: ## Run test coverage suite
	@go test ./... --coverprofile=cov.out
	@go tool cover --html=cov.out

build: ## Builds the CLI
	@mkdir -p execs
	@CGO_ENABLED=${CGO_ENABLED} go build ${GO_FLAGS} \
	-ldflags "-w -s -X ${PACKAGE}/cmd.version=${VERSION} -X ${PACKAGE}/cmd.commit=${GIT_REV} -X ${PACKAGE}/cmd.date=${DATE}" \
	-tags=${GO_TAGS} -o ${OUTPUT_BIN} main.go
	@echo "\033[32m✓\033[0m Binary built successfully: $(OUTPUT_BIN)"
	@echo "\033[34mRun it with: ./$(OUTPUT_BIN) [options]\033[0m"

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: test cover build help
