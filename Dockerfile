FROM golang:1.24-bookworm

WORKDIR /workspace

COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go mod download

COPY . .

CMD ["go", "run", "cmd/query-sniper/main.go"