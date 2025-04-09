# Query Sniper

This project is a simple daemon that watches MySQL databases and detects if they are lagging; if they are, it will search for and kill and long running queries that might be impacting the database(s).

## Design concept

Extremely simplified pseudocode for the concept:

```
every S seconds
    if (hllSize > H entries) || (replicaLag() > L seconds)gf
        find all queries that are aged > Q seconds
            kill each query
            log query digest (ie the parameterized version of a query) and the action taken
    else
        // nothing to do
        next
```

It should be noted that replica lag fetched from the database via `SHOW SLAVE STATUS` isn't the entire picture, because it doesn't take into account binlog fetching (Alex has more context); for now it will have to suffice.

## Development

Start the local mysql cluster:

1. `cd .devcontainer`
1. `docker compose down -v && docker compose up`
1. `./bootstrap.sh`

The docker compose file starts up a primary and secondary mysql instance with GTID replication configured. The `bootstrap.sh` file then:
1. On the primary server, sets the `test` user up with replication premissions and creates a sample table in the `test` schema
1. On the replica server, configures replication from the primary, and starts the replication process

Once the bootstrap script is run, you should be able to see the `test.config` table on the replica server.

## MVP

1. ✅ Watch multiple databases at a time
1. ✅ Configurable via config files (one for config, one for database auth creds)
1. ✅ Make the lag detection (HLL and replica) optional per database, via a configuration option
    - This should just kill long running queries without checking for anything impacting the database first
1. Logging when a connection is killed
    - We might also want to include the digest of said connection, if we can strip out the PII

## Long term

1. Post to slack when a query is detected and killed
1. Pagerduty alert when a query is detected and killed

## References

* The percona toolkit tool [pt-kill](https://github.com/percona/percona-toolkit/blob/3.x/bin/pt-kill); it's 9000000 lines of perl and way more than we need here
* [mqs](https://github.com/StephaneBunel/mqs) a simple query sniper; it hasn't been updated in 8 years
* [go-kill-mysql-query](https://github.com/mugli/go-kill-mysql-query) TUI tool to interactively kill queries; it's neat, but we want our process to be automated
