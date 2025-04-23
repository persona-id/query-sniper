FROM alpine:3.21.3

RUN apk add --no-cache bash=5.2.37-r0 \
    && addgroup -S sniper \
    && adduser -S sniper -u 1000 -G sniper

# config.yaml should be a configmap mounted as /app/config.yaml
# credentials.yaml should be a secret mounted as /app/credentials.yaml (or whatever path is configured in config.yaml)
COPY --chown=root:root --chmod=755 query-sniper /app/

WORKDIR /app

USER sniper

ENTRYPOINT ["/app/query-sniper"]
