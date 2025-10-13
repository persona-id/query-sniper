# syntax=docker/dockerfile:1
# This is the dockerfile that the devcontainer uses to build the container image for local development and testing.

# Stage 1
FROM golang:1.25.2-alpine AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED="0"

# Set destination for COPY
WORKDIR /build

COPY . .

RUN go mod download \
  && go build -o query-sniper cmd/query-sniper/main.go

# Stage 2
FROM alpine:3.22.1 AS runner

RUN apk add --no-cache bash=5.2.37-r0 \
  && addgroup sniper \
  && adduser -S sniper -u 1000 -G sniper

WORKDIR /app

COPY --chown=sniper:sniper --from=builder --chmod=700 /build/query-sniper /app/

USER sniper

ENTRYPOINT ["/app/query-sniper"]
