# This Dockerfile builds the binary, compresses it with upx, and then builds the final image.
# It is not used for development or anything, at the moment.

FROM golang:1.25.2-alpine AS builder

ENV GO111MODULE=on
ENV CGO_ENABLED=0

WORKDIR /build

COPY . .

RUN go mod download && \
    go build -ldflags "-s -w" -trimpath -o query-sniper cmd/query-sniper/main.go

# Stage 2 - Compress the binary with upx
FROM gruebel/upx:latest AS upx

COPY --from=builder /build/query-sniper /query-sniper.org

RUN upx --best --lzma -o /query-sniper /query-sniper.org

# Stage 3 - Final image
FROM alpine:3.22.1

RUN apk add --no-cache ca-certificates bash=5.2.37-r0 \
    && addgroup -S sniper \
    && adduser -S sniper -u 1000 -G sniper \
    && mkdir -p /app/configs \
    && chown -R sniper:sniper /app/configs

# We are explictly copying the config.yaml and credentials.yaml files into the container, although in reality:
#   - config.yaml should be a configmap mounted as /app/configs/config.yaml
#   - credentials.yaml should be a secret mounted as /app/configs/credentials.yaml (or whatever path is configured in config.yaml)
COPY --from=upx     --chown=sniper:sniper --chmod=700 /query-sniper                   /app/
COPY --from=builder --chown=sniper:sniper --chmod=600 /build/configs/config.yaml      /app/configs/
COPY --from=builder --chown=sniper:sniper --chmod=600 /build/configs/credentials.yaml /app/configs/

WORKDIR /app

USER sniper

CMD ["./query-sniper"]
