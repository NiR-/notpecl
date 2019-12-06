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
	go build -o .bin/notpecl -buildmode pie -ldflags "\
		-w -s \
		-X 'github.com/NiR-/notpecl/cmd.releaseVersion=$(VERSION)' \
		-X 'github.com/NiR-/notpecl/cmd.commitHash=$(GIT_SHA1)' \
	" .

.PHONY: test
test:
	# A whole PHP extension is build during tests, so it can be a bit slow.
	$(GOTEST) -timeout 60s -cover -coverprofile cover.out ./...
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
