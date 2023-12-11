# Copyright(C) 2023 TBD
# SPDX-License-Identifier: TBD

FROM golang:1.21-bookworm as builder

ARG CGO_ENABLED=0
ARG GOAMD64=v3
ARG GOARM=7
ARG GOFLAGS=-trimpath -mod=readonly -asmflags=all="-spectre=all" -gcflags=all="-spectre=all" -ldflags="-s -w"
ARG GOOS=linux

WORKDIR /src

COPY go.* .
RUN --mount=type=cache,target=/root/go/pkg/mod go mod download -x

COPY . .
RUN --mount=type=cache,target=/root/go/pkg/mod go build ${GOFLAGS} -o bin/exporter ./cmd/exporter


FROM scratch

COPY --from=builder /src/bin/exporter /usr/local/bin/
COPY --from=builder /src/LICENSE /usr/local/share/doc/licenses/

USER 1000

ENTRYPOINT [ "/usr/local/bin/exporter" ]
