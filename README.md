# Query Sniper

This project is a simple daemon that watches MySQL databases and detects long running queries; any that are found will be killed.

## Development

### Devcontainer

1. When prompted by vscode or intellij relaunch the project in the devcontainer.

This will use the docker compose file with an overlay to start a primary database, database replica, generate tls
certificates, and a development container. The development container shares the same network as the database instances
and can be used to develop, test, build, and run query-sniper.

When using the devcontainer the db replica bootstrap script is automatically run to establish mysql replication.

### Docker Compose
Start the local mysql cluster:

1. `docker compose down -v && docker compose up -d`
1. `docker compose exec -it app /workspace/.devcontainer/bin/bootstrap.sh`

The docker compose file starts up a primary and secondary mysql instance with GTID replication configured. The `bootstrap.sh` file then:
1. On the primary server, sets the `test` user up with replication premissions and creates a sample table in the `test` schema
1. On the replica server, configures replication from the primary, and starts the replication process

Once the bootstrap script is run, you should be able to see the `test.config` table on the replica server.

## MVP

1. ✅ Watch multiple databases at a time
1. ✅ Configurable via config files (one for config, one for database auth creds)
1. Logging when a connection is killed
    - We might also want to include the digest of said connection, if we can strip out the PII

## Long term

1. Post to slack when a query is detected and killed
1. Pagerduty alert when a query is detected and killed

## References

* The percona toolkit tool [pt-kill](https://github.com/percona/percona-toolkit/blob/3.x/bin/pt-kill); it's 9000000 lines of perl and way more than we need here
* [mqs](https://github.com/StephaneBunel/mqs) a simple query sniper; it hasn't been updated in 8 years
* [go-kill-mysql-query](https://github.com/mugli/go-kill-mysql-query) TUI tool to interactively kill queries; it's neat, but we want our process to be automated
