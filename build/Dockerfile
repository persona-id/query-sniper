# This is the dockerfile that the release process uses to build the container image.
# It is not used for development.

FROM alpine:3.22.1

RUN apk add --no-cache bash=5.2.37-r0 \
    && addgroup -S sniper \
    && adduser -S sniper -u 1000 -G sniper \
    && mkdir -p /app/configs \
    && chown -R sniper:sniper /app/configs

# We are explictly NOT copying the config.yaml file into the container.
#   - config.yaml should be a configmap mounted as /app/configs/config.yaml
#   - credentials.yaml should be a secret mounted as /app/configs/credentials.yaml (or whatever path is configured in config.yaml)
COPY --chown=sniper:sniper --chmod=700 query-sniper /app/

WORKDIR /app

USER sniper

ENTRYPOINT ["/app/query-sniper"]
