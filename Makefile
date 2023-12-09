SHELL := /bin/bash

# The name of the executable
TARGET := 'query-sniper'

# Use linker flags to provide version/build settings to the target.
LDFLAGS=-ldflags "-s -w"

all: clean lint build

$(TARGET):
	@go build $(LDFLAGS) -o $(TARGET) cmd/query-sniper/main.go

build: clean $(TARGET)
	@true

clean:
	@rm -rf $(TARGET) *.test *.out tmp/* coverage dist

lint:
	@gofumpt -l -w .
	@go vet ./...
	@golangci-lint run --config=.golangci.yml --allow-parallel-runners

tests:
	@mkdir -p coverage
	@go test ./... -v -shuffle=on -coverprofile coverage/coverage.out

coverage: test
	@go tool cover -html=coverage/coverage.out

run: build
	@./$(TARGET)

docker: clean lint
	@docker build -f build/dev.Dockerfile . -t persona-id/query-sniper:latest

snapshot: clean lint
	@goreleaser --snapshot --clean

release: clean lint
	@goreleaser --clean
