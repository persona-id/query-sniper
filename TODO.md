# General TODOs

## Version 0.1.0 - MVP

- ✅ Watch multiple databases at a time
- ✅ Configurable via config files (one for config, one for database auth creds)
- ✅ Databases should have independent, configurable times for long queries
- ✅ Better use of contexts and signal handling
- ✅ More detailed logging when a query/txn is killed
- ✅ Improve test coverage
- ✅ Use the new [synthetic time](https://antonz.org/go-1-25/#synthetic-time-for-testing) in the tests
- ✅ Use the digest of long queries/txns, in order to strip out any potential PII
- ✅ Add `AND STATE NOT IN ('cleaning up')` filter to the hunting query, as it's harmless
- ✅ Exclude `ALTER` and other DDL commands; focus only on CRUD commands for killing
- ✅ Long transaction (txn) detection and killing
- Use DB mocking in tests, so that we can actually test the SQL commands
  - Or maybe look into testcontainers, and run an actual MySQL instance against which we are running integration tests
- Copy long query time from web into the settings
- See if the sniper can detect the `MYSQL_TIMEOUT` (or whatever it is) query hint and abide by that setting rather than the default
- Expose metrics as an http endpoint, at least the stock golang metrics via the prometheus library

## Longer Term Features

- Post to Slack when a query/txn is detected (and/or killed)
  - This will require extra configuration, and should be entirely optional
- Fire a Pagerduty alert when a query/txn is detected (and/or killed)
  - This will also require extra configuration, and should be entirely optional
- Post something or other to Datadog when a query/txn is detected (and/or killed)
  - An event?
  - A metric?
- Statsig integration? Might be overkill for us, and we if we DO pursue it, it'd have to be completely optional
  - We could create a persona plugin, I suppose, but that adds some complexity and it isn't super OSS friendly
- If marginalia comments ever return, we should extract the comment from the query and add it to the `slog` output, for easier tracing
