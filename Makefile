SHELL := /bin/bash

# The name of the executable
TARGET := 'query-sniper'

# Use linker flags to provide version/build settings to the target.
LDFLAGS=-ldflags "-s -w"

.PHONY: all build clean lint test coverage run docker snapshot release

all: lint build

$(TARGET):
	@go build $(LDFLAGS) -o $(TARGET) cmd/query-sniper/main.go

build: clean $(TARGET)
	@true

clean:
	@rm -rf $(TARGET) *.test *.out tmp/* coverage dist

lint:
	@go tool gofumpt -l -w .
	@golangci-lint run --config=.golangci.yml --allow-parallel-runners # includes govet and gosec

test:
	@mkdir -p coverage
	@go test ./... -v -shuffle=on -coverprofile coverage/coverage.out

coverage: test
	@go tool cover -html=coverage/coverage.out

run: build
	@./$(TARGET)

docker: clean lint
	@docker build -f build/dev.Dockerfile . -t persona-id/query-sniper:latest

snapshot: clean lint
	@go tool goreleaser --snapshot --clean

release: clean lint
	@go tool goreleaser --clean
