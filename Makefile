export GO111MODULE=on

# Either use `gotest` if available (same as `go test` but with colors), or use
# `go test`.
GOTEST := go test
ifneq ($(shell which gotest),)
	GOTEST := gotest
endif

.PHONY: build
build:
	go build -buildmode pie -ldflags '-w -s' -o .bin/notpecl .

.PHONY: test
test:
	# A whole PHP extension is build during tests, so it can be a bit slow.
	$(GOTEST) -timeout 60s -cover -coverprofile cover.out ./...
	go tool cover -o cover.html -html=cover.out

.PHONY: gendoc
gendoc: build
	./.bin/notpecl gendoc --dest ./docs
