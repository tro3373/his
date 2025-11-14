SHELL := bash
mkfile_path := $(abspath $(lastword $(MAKEFILE_LIST)))
mkfile_dir := $(patsubst %/,%,$(dir $(mkfile_path)))
PATH := $(mkfile_dir)/bin:$(PATH)
.SHELLFLAGS := -eu -o pipefail -c # -c: Needed in .SHELLFLAGS. Default is -c.
.DEFAULT_GOAL := run

ifndef GOBIN
GOBIN := $(shell echo "$${GOPATH%%:*}/bin")
endif

COBRA := $(GOBIN)/cobra

$(COBRA): ; @go get -v -u github.com/spf13/cobra/cobra

.PHONY: get
get:
	@go get github.com/mitchellh/go-homedir \
		go get github.com/spf13/viper

dst := his
all: clean gen tidy fmt lint build test
clean:
	@echo "==> Cleaning" >&2
	@rm -f $(dst)
	@go clean -cache -testcache
tidy:
	@echo "==> Running go mod tidy -v"
	@go mod tidy -v
tidy-go:
	@v=$(shell go version|awk '{print $$3}' |sed -e 's,go\(.*\)\..*,\1,g') && go mod tidy -go=$${v}
deps:
	@go list -m all
update:
	@go get -u ./...
fmt:
	@echo "==> Running go fmt ./..." >&2
	@go fmt ./...
lint:
	@echo "==> Running golangci-lint run" >&2
	@golangci-lint run

build: build-linux-amd
build-linux-arm: _build-linux-arm64
build-linux-amd: _build-linux-amd64
build-android-arm: _build-android-arm64
build-android-amd: _build-android-amd64
build-darwin-arm: _build-darwin-arm64
build-darwin-amd: _build-darwin-amd64
build-windows-arm: _build-windows-arm64
build-windows-amd: _build-windows-amd64
# CGO_ENABLED=0: Disable CGO
# -trimpath: Remove all file system paths from the resulting executable.
# -s: Omit the symbol table and debug information.
# -w: Omit the DWARF symbol table.
_build-%: clean lint
	@echo "==> Go Building" >&2
	$(eval goos=$(firstword $(subst -, ,$*)))
	$(eval goarch=$(word 2, $(subst -, ,$*)))
	@env GOOS=$(goos) GOARCH=$(goarch) CGO_ENABLED=0 \
		go build -v \
			-trimpath \
			-ldflags="-s -w" \
			-o $(dst) \
			.

help:
	@go run . --help
run:
	@go run . $(arg)
latest:
	@go run . latest $(arg)
tag:
	@go run . tag $(arg)

pkg := ./...
cover_mode := atomic
cover_out := cover.out
test: testsum-cover-check
# test-normal:
# 	@echo "==> Testing $(pkg)" >&2
# 	@go test -v $(pkg)
# test-cover:
# 	@echo "==> Running go test with coverage check" >&2
# 	@go test $(pkg) -coverprofile=$(cover_out) -covermode=$(cover_mode) -coverpkg=$(pkg)
# test-cover-count:
# 	@echo "==> Running go test with coverage check (count mode)" >&2
# 	@make test-cover cover_mode=count
# 	@go tool cover -func=$(cover_out)
# test-cover-html: test-cover
# 	@go tool cover -html=$(cover_out) -o cover.html
# test-cover-open: test-cover
# 	@go tool cover -html=$(cover_out)
# test-cover-check: test-cover-html
# 	@echo "==> Checking coverage threshold" >&2
# 	@go-test-coverage --config=./.testcoverage.yml
testsum:
	@echo "==> Running go testsum" >&2
	@gotestsum --format testname -- -v $(pkg) -coverprofile=$(cover_out) -covermode=$(cover_mode) -coverpkg=$(pkg)
testsum-cover-check: testsum
	@echo "==> Running test-coverage" >&2
	@go-test-coverage --config=./.testcoverage.yaml


gen: mockery swag
swag:
	@echo "==> Running swag init" >&2
	@swag init -g cmd/main.go -o swagger --parseInternal --parseDependency --parseDependencyLevel 1
mockery:
	@echo "==> Running mockery" >&2
	@mockery

gr_init:
	@goreleaser init
gr_check:
	@goreleaser check
gr_snap:
	@goreleaser release --snapshot --clean $(OPT)
gr_snap_skip_publish:
	@OPT=--skip-publish make gr_snap
gr_build:
	@goreleaser build --snapshot --clean

