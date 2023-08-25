# Copyright(C) 2023 Bryon Nevis
# SPDX-License-Identifier: Apache 2.0

GO=CGO_ENABLED=0 go
GOFLAGS=-trimpath -mod=readonly

.PHONY: cmd/exporter/exporter

cmd/exporter/exporter: 
	GOOS=linux $(GO) build $(GOFLAGS) -o $@ ./cmd/exporter

run:
	./cmd/exporter/exporter

docker:
	docker build -t docker.io/bnevis/random_exporter:latest .

docker_push:
	docker push docker.io/bnevis/random_exporter:latest

clean:
	rm -f cmd/exporter/exporter
