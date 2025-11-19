# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.6] - 2025-11-19

- **Transaction Detection/Killing**: Enables long running txn detection and handling (also supports dry run/safe mode)
- **Query Tuning**: Will now only detect CRUD operations (`INSERT`, `SELECT`, `UPDATE`, `DELETE`), and will no longer detect or kill DDL changes
- **Dependency Updates**: Upgraded to the latest `x/sys` and `x/text` libraries
- **Improved Testing**: Fixed and updates a few tests to increase coverage; without DB mocking, we can't really test the actual DB calls yet

## [0.1.5] - 2025-09-26

- **Logging Improvements**: Added the database name to the first line of the log message, for a better view in the datadog log tail
- **Dependency Updates**: Upgraded transitive depedencies
- **golangci-lint Update**: Upgraded to the latest golangci-lint, and addressed the new failing linters in the code

## [0.1.4] - 2025-09-17

### Changed
- **Logging Improvements**: Don't log the long query and long transaction queries that snipers use to `INFO`. Only `DEBUG` or `TRACE` will show the specific queries now
- **Keep a Changelog format**: Update CHANEGLOG.md to support the 1.1.0 standard of keepachangelog

## [0.1.3] - 2025-09-12

Added SSL support for SQL connections.

### Added
- **SSL Support**: Support SSL connections to MySQL backends, using the configuration file
- **SSL Certificate Validation**: Add comprehensive SSL configuration validation on boot, in order to prevent insecure or invalid certificate combinations
- **Test Coverage Expansion**: Added comprehensive SSL validation test suite

### Changed
- **Configuration Validation Improvements**: Extended validation system with new error types
- **Updated Documentation**: README improvements with detailed SSL configuration guidance

## [0.1.2] - 2025-09-10

### Fixed
- **Configuration Bug Fixes**: Fix some issues with safe-mode flag handling and test reliability

## [0.1.1] - 2025-09-05

### Added
- **Long Running Transactions**: add long-running transaction detection, in addition to the existing query detection
- **Dry Run**: refactor `dry_run` from the global config to a per-database configuration
- **Safe Mode**: implements the `--safe-mode` CLI flag for global dry-run override capability
  - Adds comprehensive precedence logic (safe-mode overrides per-db settings)

### Changed
- **Enhance Logging**: includes transaction details and override status visibility
- **Test Coverage**: add test coverage for all new features and precedence scenarios
- **CI**: improve build system with semgrep security scanning and dependency updates

## [0.1.0] - 2025-09-02

Initial MVP release.

### Added
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
