# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Query Sniper is a high-performance MySQL query monitoring daemon written in Go 1.25. It automatically detects and optionally terminates long-running queries and transactions across multiple database instances.

### Key Architecture Components

- **cmd/query-sniper/main.go**: Entry point with signal handling and graceful shutdown
- **internal/configuration/**: Configuration management using Viper with separate credential files
- **internal/sniper/**: Core sniper logic with concurrent database monitoring

The application uses a concurrent architecture where each configured database gets its own goroutine running an independent sniper loop.

## Development Commands

### Building and Testing
```bash
make build          # Build the binary
make test           # Run tests with race detection
make coverage       # Generate coverage report
make coverage-html  # Generate and open HTML coverage report
make bench          # Run benchmarks
```

### Code Quality
```bash
make fmt            # Format code using golangci-lint
make lint           # Run golangci-lint with auto-fix
make semgrep        # Run semgrep security scan
make check          # Run all quality checks (fmt, lint, semgrep, test)
```

### Running the Application
```bash
make run            # Smart run - detects if in devcontainer vs local
./query-sniper      # Direct execution with default config

# With custom config files:
SNIPER_CONFIG_FILE=path/to/config.yaml SNIPER_CREDS_FILE=path/to/creds.yaml ./query-sniper

# Command line overrides:
./query-sniper --dry-run=true --log.format=text --log.level=debug
```

### Development Environment
The project includes a complete devcontainer setup with MySQL GTID replication. Open in VSCode/Cursor or use:
```bash
devcontainer up --workspace-folder .
devcontainer exec --workspace-folder . bash
```

## Configuration Architecture

Query Sniper uses a dual-file configuration system:

1. **Main Config** (`configs/config.yaml`): Database definitions, intervals, limits
2. **Credentials** (`configs/credentials.yaml`): Separate username/password file for security. This file is meant to be stored in a secret manager (AWS, GCP, Vault, etc) and mounted to the pods in which the aplication is running

The configuration system uses Viper with pflag integration for CLI overrides. Environment variables `SNIPER_CONFIG_FILE` and `SNIPER_CREDS_FILE` can override file paths.

## Core Sniper Logic

Each sniper instance:
- Connects to a MySQL database using `go-sql-driver/mysql`
- Generates templated SQL queries for long-running query and transaction detection
- Runs periodic checks using `performance_schema.processlist` and `information_schema.innodb_trx`
- Logs info about long running queries exceeding the thresholds
- Optionally executes `KILL` commands on processes exceeding thresholds
- Uses structured logging with slog, avoiding PII by logging digest_text instead of raw queries

The sniper uses Go 1.25's new `sync.WaitGroup.Go()` method for concurrent execution.

## Testing and Linting

- Uses `golangci-lint-v2` (note the v2 suffix in Makefile)
- Tests must always run with `--race` and `--shuffle=on` flags
- Semgrep for security scanning
- Field alignment linting enforced (structs sorted by datatype)

## Important Implementation Notes

- Long transaction detection is recently added (see git branch `kuzmik/long-txn-detection`)
- Dry run mode exists but is not fully implemented in the kill logic
- Signal handling supports graceful shutdown on `SIGINT`/`SIGTERM`
- Database grants for the MySQL user that the appliaction uses must have `PROCESS` and `CONNECTION_ADMIN` (MySQL 8+) or `SUPER` privileges as well as access to the `performance_schema` and `information_schema` tables. See `README.md` or `.devcontainers/bin/bootstrap-dbs.sh` for more details
- Schema filtering is per-database configurable but currently single-schema only
