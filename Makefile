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
	@go get github.com/mitchellh/go-homedir \
		go get github.com/spf13/viper

# .PHONY: init-gen
# init-gen: $(COBRA)
# 	@go mod init $(PACKAGE) \
# 	&& $(COBRA) init --pkg-name $(PACKAGE)
#
# .PHONY: add-hello
# add-hello: $(COBRA)
# 	@$(COBRA) add hello

.PHONY: deps
deps:
	@go list -m all

.PHONY: tidy
tidy:
	@go mod tidy

.PHONY: build
build:
	@env GOOS=linux go build -ldflags="-s -w"

# .PHONY: clean
# clean:
# 	rm -rf ./bin logs stress

#
# .PHONY: help
# help:
# 	@go run ./main.go --help
#
# .PHONY: front
# front:
# 	@go run ./main.go front
#
# .PHONY: back
# back:
# 	@go run ./main.go back
#
# .PHONY: reguser
# reguser:
# 	@go run ./main.go reguser

.PHONY: run
run:
	@go run ./main.go
