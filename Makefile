# @see
# https://qiita.com/Syoitu/items/8e7e3215fb7ac9dabc3a
# https://qiita.com/keitakn/items/f46347f871083356149b
NAME := his
VERSION := v0.0.1
# REVISION := $(shell git rev-parse --short HEAD)
# OSARCH := "darwin/amd64 linux/amd64"
PACKAGE := github.com/tro3373/$(NAME)
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

.PHONY: deps
deps:
	@go list -m all

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: build
build:
	@env GOOS=linux go build -ldflags="-s -w"

.PHONY: install
install:
	@go install

.PHONY: clean
clean:
	rm -rf ./$(NAME)

.PHONY: help
help:
	@go run ./main.go --help

.PHONY: run
run:
	@go run ./main.go latest
