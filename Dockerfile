FROM golang:1.24-bookworm

RUN apt-get update && apt-get install -y \
  ca-certificates \
  curl \
  default-mysql-client \
  iputils-ping \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY go.* ./
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  go mod download

COPY . .

CMD ["go", "run", "cmd/query-sniper/main.go"]
