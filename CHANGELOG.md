## 0.1.1 - 2025-09-05

- **Long Running Transactions**: add long-running transaction detection, in addition to the existing query detection
- **Dry Run**: refactor `dry_run` from the global config to a per-database configuration
- **Safe Mode**: implements the `--safe-mode` CLI flag for global dry-run override capability
  - Adds comprehensive precedence logic (safe-mode overrides per-db settings)
- **Enhance Logging**: includes transaction details and override status visibility
- **Test Coverage**: add test coverage for all new features and precedence scenarios
- **CI**: improve build system with semgrep security scanning and dependency updates

## 0.1.0 - 2025-09-02

Initial MVP release, with the following features:

- **Multi-Database Monitoring**: Monitor multiple MySQL instances simultaneously with independent configurations
- **Intelligent Query Detection**: Uses MySQL's `performance_schema.processlist` for efficient long-running query identification
- **Automated Query Termination**: Automatically kills queries exceeding configurable time thresholds using MySQL's `KILL` command
- **Per-Database Configuration**: Independent intervals, query limits, and transaction limits for each database instance
- **Schema-Specific Monitoring**: Optional schema filtering to limit monitoring to specific databases
- **Dry Run Mode**: Test configurations and detect long-running queries without actually terminating them
- **Graceful Process Filtering**: Automatically excludes sleeping connections, killed processes, internal MySQL processes, and `performance_schema` queries
- **Dual Configuration Files**: Separate main config (`config.yaml`) and credentials file (`credentials.yaml`) for security
- **YAML Configuration**: Human-readable YAML format with support for anchors and references (`&default_config`, `<<: *default_config`)
- **Environment Variable Support**:
  - `SNIPER_CONFIG_FILE`: Override main config file path
  - `SNIPER_CREDS_FILE`: Override credentials file path
- **Command Line Overrides**: CLI flags for all major settings:
  - `--log.level`: Override log level
  - `--log.format`: Override log format (`JSON`, `TEXT`)
  - `--log.include_caller`: Include source code caller information
  - `--dry-run`: Enable dry run mode
  - `--show-config`: Display redacted configuration and exit
- **Configuration Validation**: Comprehensive validation with descriptive error messages
- **Password Redaction**: Automatic password masking in configuration display and logs
- **Structured Logging**: Built on Go's `slog` with structured key-value logging
- **Multiple Log Levels**: Support for the normal slog levels like `DEBUG`, `INFO`, `WARN`, `ERROR`, but adds `TRACE` and `FATAL` custom level definitions
- **Dual Output Formats**:
  - **JSON Format**: Machine-readable structured logs for production
  - **Text Format**: Human-readable colorized output for development with `tint` library
- **Caller Information**: Optional source code location tracking (`include_caller`)
- **RFC3339 Timestamps**: Standardized time format across all log entries
- **Build Information Logging**: Automatic logging of Go version, module info, build settings, and compilation details
- **Multi-Stage Docker Build**: Optimized Docker images with build caching
- **Container Optimizations**: Proper signal handling for container orchestration
- **VSCode/Cursor Devcontainer**: Complete development environment with:
  - Go 1.25+ toolchain pre-installed
  - MySQL replication cluster (primary + 2 replicas) with GTID
  - TLS certificate generation and management
  - Automatic database bootstrapping
  - Pre-configured VS Code extensions (Go, YAML, GitBlame)
- **Docker Compose Stack**: Full development infrastructure including:
  - Primary MySQL instance (port 3306)
  - Two replica instances with GTID replication
  - Certificate generation service
  - Development container with mounted source
- **Database Utilities**: Custom `db` script for database interactions
- **Automated Setup**: Post-creation scripts for dependency installation and database initialization
- **Comprehensive Linting**: golangci-lint v2 with 80+ enabled linters including:
  - All default linters enabled except specific exclusions
  - Dependency management with depguard
  - Code formatting with gofumpt extra rules
  - Import organization with local prefix support
- **Test Suite**:
  - Race condition detection (`--race` flag)
  - Test randomization (`--shuffle=on` flag)
- **Coverage Reporting**: HTML and terminal coverage reports with detailed metrics
- **Go 1.25 Features**: Uses latest Go features including new `wg.Go()` syntax for wait groups
- **Concurrent Database Monitoring**: Each database instance monitored in separate goroutine
- **Graceful Shutdown**: Proper context cancellation and signal handling
- **Signal Support**: Handles `SIGINT`, `SIGTERM`, `SIGUSR1`, `SIGUSR2`, `SIGHUP` with appropriate responses
- **Connection Pooling**: Efficient database connection management
- **Template-Based Query Generation**: Dynamic SQL generation with Go templates for flexible filtering
- **Credential Separation**: Database credentials stored separately from main configuration
- **Secret Manager Ready**: Designed for integration with Google Cloud Secret Manager and similar systems
- **Password Protection**: Automatic credential redaction in logs and configuration dumps
- **Minimal Privileges**: Requires only `PROCESS` and `CONNECTION_ADMIN`/`SUPER` MySQL privileges
- **No Credential Logging**: Sensitive information never appears in log output
- **Comprehensive README**: Complete setup, configuration, and deployment documentation
- **Configuration Examples**: Multiple configuration examples for different environments
- **Make Target Help**: Built-in help system showing all available commands
- **Performance Schema Dependency**: Requires `performance_schema` to be enabled
- **Query Information Logging**: Detailed process information capture (ID, user, state, command, execution time, query text)
- **Detailed Process Logging**: Comprehensive logging of killed processes including:
  - Database name and process ID
  - User and execution time
  - Process state and command type
  - Query text, with PII protection via logging `digest_text` instead of `INFO`
- **Error Resilience**: Continues processing other queries if individual kills fail
- **Build Info Tracking**: Logs build information for debugging and deployment tracking
- **Audit Trail**: Complete audit trail of all actions for compliance and troubleshooting

## Unreleased

- Basic functionality
