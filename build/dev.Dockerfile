# syntax=docker/dockerfile:1

# Stage 1
FROM golang:1.21.5-alpine AS builder

ARG BUILD_SHA
ARG BUILD_TIME
ARG VERSION

ENV GO111MODULE=on

# Set destination for COPY
WORKDIR /build

COPY go.sum go.mod ./

RUN go mod download

COPY . .

RUN CGO_ENABLED="0" go build -o query-sniper cmd/query-sniper/main.go

# Stage 2
FROM alpine:3.18.4 as runner

RUN addgroup sniper && adduser -S sniper -u 1000 -G sniper

RUN apk add --no-cache bash

WORKDIR /app

# FIXME: missing the configs, I haven't decided how to deal with that yet.
COPY --chown=sniper:sniper --from=builder --chmod=700 /build/query-sniper /app/

USER sniper

ENTRYPOINT ["/app/query-sniper"]