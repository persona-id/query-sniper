# Query Sniper

A lightweight, high-performance MySQL query monitoring daemon that automatically detects and terminates long-running queries across multiple database instances.

[![Go Report Card](https://goreportcard.com/badge/github.com/persona-id/query-sniper)](https://goreportcard.com/report/github.com/persona-id/query-sniper)

## Features

- **Multi-Database Monitoring**: Monitor multiple MySQL instances simultaneously
- **High Performance**: Built with Go 1.25 and concurrent processing
- **Configurable Thresholds**: Per-database query and transaction time limits
- **SSL/TLS Support**: Flexible SSL configuration supporting both CA-only and mutual TLS modes
- **Dry Run Mode**: Detect long running queries or transactions without actually killing them
- **Structured Logging**: JSON and human-readable log formats with slog
- **Secure Configuration**: Separate credential files for security; meant to be stored in a secret manager like Google Cloud Secret Manager or the like
- **Container Ready**: Docker and Kubernetes deployment support
- **Performance Schema**: Uses MySQL's `performance_schema.processlist` for efficient query detection

## How It Works

Query Sniper connects to your MySQL databases and periodically queries the `performance_schema.processlist` table to identify long-running queries and transactions. If nor running in dry run mode, queries are automatically terminated using MySQL's `KILL` command when they exceed the configured thresholds.

The tool intelligently filters out:
- Sleeping connections
- Already killed processes
- Internal MySQL processes
- `performance_schema` queries

The MySQL user that the application uses to connect to the instance(s) needs to have `PROCESS` and `CONNECTION_ADMIN` (on MySQL 8+, lower than that requires `KILL` instead) to function properly.

```sql
CREATE USER IF NOT EXISTS 'sniper'@'%' IDENTIFIED WITH 'caching_sha2_password' BY 'sniper';
GRANT PROCESS, CONNECTION_ADMIN ON *.* TO 'sniper'@'%';
FLUSH PRIVILEGES;
```

_NB_: the sniper user can NOT kill queries owned by `root`.

## Requirements

- Go 1.25+ (for building from source)
- MySQL 5.7+ with Performance Schema enabled
- Database user with `PROCESS` and `SUPER` (or `CONNECTION_ADMIN`) privileges

## Configuration

Query Sniper uses two YAML configuration files:

### Main Configuration (`configs/config.yaml`)

```yaml
# Path to credentials file
credential_file: configs/credentials.yaml

# Default settings applied to all databases
default_config: &default_config
  interval: 1s                   # Check frequency
  long_query_limit: 2s           # Kill queries running longer than this
  long_transaction_limit: 60s    # Kill transactions longer than this
  dry_run: false                 # Per-database dry run setting

# Database definitions
databases:
  primary:
    address: mysql-primary-01.example.com:3306
    schema: myapp                 # Optional: limit to specific schema
    # SSL/TLS configuration (optional)
    ssl_ca: /path/to/ca-cert.pem           # CA-only mode: encrypted connection
    <<: *default_config
  replica1:
    # an internal mysql instance that does not have SSL enabled
    address: mysql-replica-01.example.com:3306
    schema: myapp
    interval: 10s                 # Override default check interval
    long_query_limit: 30s         # More lenient on replica
    dry_run: true                 # Enable dry run for replica
  cloud_sql:
    address: 10.1.2.3:3306
    schema: production
    # A Google Cloud SQL instance that requires mutual TLS
    ssl_ca: /secrets/server-ca.pem
    ssl_cert: /secrets/client-cert.pem
    ssl_key: /secrets/client-key.pem
    dry_run: false                # Production database - will actually kill queries
    <<: *default_config

# Logging configuration
log:
  level: INFO                     # DEBUG, INFO, WARN, ERROR
  format: json                    # json or text
  include_caller: false
```

### Credentials File (`configs/credentials.yaml`)

The keys in `databases` MUST match the keys defined in the `databases` block of the main config.yaml file.

**Note**: SSL certificate paths are configured in the main `config.yaml`, not in the credentials file.

```yaml
databases:
  production:
    username: sniper_user
    password: secure_password
  replica:
    username: replica_user
    password: another_password
  cloud_sql:
    username: query-sniper-user
    password: cloud_sql_password
```

### Environment Variables

- `SNIPER_CONFIG_FILE`: Override config file path
- `SNIPER_CREDS_FILE`: Override credentials file path

### Command Line Options

```bash
--log.level=INFO              # Log level override
--log.format=json            # Log format override
--log.include_caller=false   # Include caller in logs
--safe-mode=false            # Enable safe mode globally (overrides all database dry_run settings)
--show-config=false          # Show the configuration and exit (passwords redacted)
```

### Safe Mode

Query Sniper supports a global safe mode feature that provides an additional safety layer:

- **Global Safe Mode** (`--safe-mode=true`): When enabled, overrides ALL individual database `dry_run` settings and forces all snipers into dry-run mode
- **Per-Database Dry Run**: Individual databases can have their own `dry_run` setting in the configuration
- **Precedence**: Global safe mode always takes precedence over individual database settings

```yaml
# Global safe mode can be set via command line
# --safe-mode=true

# Per-database dry_run settings
databases:
  production:
    dry_run: false    # Will be overridden if global safe-mode is true
    # ... other config
  staging:
    dry_run: true     # Always in dry-run mode regardless of safe-mode
    # ... other config
```

This ensures that even if individual database configurations are set to kill queries, the global safe mode provides a kill-switch to prevent any actual query termination across all databases.

## SSL/TLS Configuration

Query Sniper supports secure SSL/TLS connections to MySQL databases with two modes:

### CA-Only Mode (Recommended for most deployments)

For basic encrypted connections without client authentication:

```yaml
databases:
  mydb:
    address: mysql.example.com:3306
    ssl_ca: /path/to/ca-cert.pem
    # ssl_cert and ssl_key omitted for CA-only mode
```

**When to use:**
- AWS RDS MySQL with SSL enabled
- Standard MySQL installations with SSL
- Azure Database for MySQL
- Most cloud-hosted MySQL instances

### Mutual TLS Mode (Enhanced Security)

For environments requiring client certificate authentication:

```yaml
databases:
  secure_db:
    address: secure-mysql.example.com:3306
    ssl_ca: /path/to/server-ca.pem      # Server's CA certificate
    ssl_cert: /path/to/client-cert.pem  # Client certificate
    ssl_key: /path/to/client-key.pem    # Client private key
```

**When to use:**
- Environments with mutual TLS policies
- High-security deployments requiring client authentication

### SSL Configuration Validation Rules

Query Sniper enforces strict SSL configuration validation to ensure secure connections:

#### Valid SSL Configurations

1. **CA-Only Mode** (Encrypted connection without client auth):
   - `ssl_ca` only - Enables encrypted connection
   - Example: AWS RDS, Azure Database for MySQL

2. **Mutual TLS Mode** (Full mutual authentication):
   - All three: `ssl_ca`, `ssl_cert`, `ssl_key` - Enables mutual TLS
   - Example: Google Cloud SQL, enterprise deployments

#### Invalid SSL Configurations (Result in Unencrypted Connection)

- `ssl_cert` only - Client cert without CA validation
- `ssl_key` only - Client key without cert or CA
- `ssl_cert` + `ssl_key` (without `ssl_ca`) - Client auth without server validation
- `ssl_cert` + `ssl_ca` (without `ssl_key`) - Incomplete mutual TLS setup
- `ssl_key` + `ssl_ca` (without `ssl_cert`) - Incomplete mutual TLS setup

#### Implementation Notes

- Invalid partial combinations return an error on start
- Certificate paths must be accessible by the Query Sniper process
- For Kubernetes deployments, it's recommended to mount certificates as secrets or configmaps
- Test SSL connectivity with `mysql` client before deploying Query Sniper

## Installation & Usage

There will be a docker image on the github project page, you can pull that to simplify things.

`TODO(kuzmik): add link here once we release a version`

### Building from Source

```bash
# Clone the repository
git clone https://github.com/persona-id/query-sniper.git
cd query-sniper

# Build
make build

# Run with default config
./query-sniper

# Run with custom config
SNIPER_CONFIG_FILE=/path/to/config.yaml ./query-sniper
```

### Using Make Targets

```bash
make help           # Show available commands
make build          # Build the binary
make test           # Run tests
make lint           # Run linting
make coverage       # Generate test coverage
make clean          # Clean build artifacts
make docker         # Build Docker image
```

### Docker

```bash
# Build container
make docker

# Run container
docker run -v $(pwd)/configs:/configs persona-id/query-sniper:latest
```

## Development

### Development Environment

The project includes a complete development environment complete with MySQL GTID replication on two replicas, via [Devcontainers](https://containers.dev/).

**Option 1: Using VSCode or Cursor**
Just open the project and start the devcontainer - you're good to go.

**Option 2: Using Devcontainers CLI**
```bash
# install the devcontainers CLI (if not already installed)
npm install -g @devcontainers/cli

# start the devcontainer
devcontainer up --workspace-folder .

# execute commands in the devcontainer
devcontainer exec --workspace-folder . bash
```

**Option 3: Manual Docker Compose**

This option is more work, but should still work.

```bash
# start development environment
docker compose -f docker-compose.yml -f .devcontainer/docker-compose.devcontainer.yml up -d

# run setup scripts manually
docker compose exec devcontainer bash -i .devcontainer/bin/install-dependencies
docker compose exec devcontainer bash -i .devcontainer/bin/bootstrap-dbs.sh

# access the development container
docker compose exec devcontainer bash
```

This sets up:
- Primary MySQL instance (port 3306)
- Two replica instances with GTID replication
- TLS certificates for secure connections
- Development container with Go toolchain

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
make coverage

# View HTML coverage report
make coverage-html

# Run benchmarks
make bench
```

### Code Quality

The project uses comprehensive linting with golangci-lint:

```bash
# Run linting
make lint

# Auto-fix formatting issues
make fmt

# Run all quality checks
make check
```

## Deployment

### Database Permissions

Create a dedicated user for Query Sniper:

```sql
-- Create user and grant access to the two extra tables we need
CREATE USER 'sniper'@'%' IDENTIFIED BY 'secure_password';
GRANT SELECT ON performance_schema.threads TO 'sniper'@'%';
GRANT SELECT ON performance_schema.events_statements_current TO 'sniper'@'%';

-- For MySQL 8.0+, prefer CONNECTION_ADMIN over SUPER
GRANT CONNECTION_ADMIN, PROCESS ON *.* TO 'sniper'@'%';

FLUSH PRIVILEGES;
```

## Logging

Query Sniper provides detailed structured logging:

```json
{
  "time": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "msg": "killed mysql process",
  "db": "production",
  "process_id": 12345,
  "time": 65,
  "state": "Sending data",
  "command": "Query",
  "info": "SELECT * FROM large_table WHERE..."
}
```

## Safety Features

- **Dry Run Mode**: Test configurations without killing queries
- **Process Filtering**: Automatically excludes system processes
- **Error Handling**: Continues processing other queries if one fails
- **Structured Logging**: Full audit trail of all actions
- **Per-Database Configuration**: Fine-tuned control per instance

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass: `make check`
6. Submit a pull request

## License

[BSD-3-Clause](LICENSE)

## References

- [Percona Toolkit pt-kill](https://github.com/percona/percona-toolkit/blob/3.x/bin/pt-kill) - Feature-rich Perl alternative
- [MySQL Performance Schema](https://dev.mysql.com/doc/refman/8.0/en/performance-schema.html) - Official documentation
- [go-kill-mysql-query](https://github.com/mugli/go-kill-mysql-query) - TUI tool to interactively kill queries
