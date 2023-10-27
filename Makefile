# Copyright(C) 2023 Bryon Nevis
# SPDX-License-Identifier: Apache 2.0

GO=CGO_ENABLED=0 GOAMD64=v4 go
GOFLAGS=-trimpath -mod=readonly -asmflags="all=-spectre=all" -gcflags="all=-spectre=all" -ldflags="-s -w"

.PHONY: all build run lint clean

all: build

build: 
	GOOS=linux $(GO) build $(GOFLAGS) -o bin ./cmd/exporter

run:
	./bin/exporter

lint:
	golangci-lint run --verbose --config .golangci.yml

clean:
	rm -f bin/exporter
