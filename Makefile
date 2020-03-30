NO_CACHE ?= --no-cache
export GO111MODULE=on

# Used by go linker at build time to set the variables needed for `notpecl version`.
GIT_SHA1 := $(shell git rev-parse HEAD)
ifeq ($(VERSION),)
ifneq ($(CIRCLE_TAG),)
	VERSION := $(IMAGE_TAG)
else
	VERSION := dev version
endif
endif


# Either use `gotest` if available (same as `go test` but with colors), or use
# `go test`.
GOTEST := go test
ifneq ($(shell which gotest),)
	GOTEST := gotest
endif

.PHONY: build
build:
	docker build $(NO_CACHE) \
		-f Dockerfile.build \
		-t notpecl \
		--build-arg "VERSION=$(VERSION)" \
		--build-arg "COMMIT_HASH=$(GIT_SHA1)" \
		.
	docker run --rm \
		-v $(PWD)/.bin:/mnt \
		notpecl \
		cp notpecl /mnt/notpecl

.PHONY: test
test:
	$(GOTEST) -cover -coverprofile cover.out ./...
	go tool cover -o cover.html -html=cover.out

.PHONY: gendoc
gendoc: build
	./.bin/notpecl gendoc --dest ./docs

.PHONY: .release
.release:
ifeq ($(GIT_TAG),)
	$(error You have to provide the GIT_TAG of the release)
endif
ifeq ($(NOTPECL_BIN),)
	$(error You have to provide the path to notpecl binary)
endif
	./.circleci/upload_bin_to_github

.PHONY: gen-mocks
gen-mocks:
	go generate ./...
