export GO111MODULE=on

# Either use `gotest` if available (same as `go test` but with colors), or use
# `go test`.
GOTEST := go test
ifneq ($(shell which gotest),)
	GOTEST := gotest
endif

.PHONY: build
build:
	go build -buildmode pie -ldflags '-w' -o notpecl .

.PHONY: test
test:
	# A whole PHP extension is build during tests, so it can be a bit slow.
	$(GOTEST) -timeout 60s ./...

.PHONY: gendoc
gendoc: build
	./notpecl gendoc --dest ./docs
