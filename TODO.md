# TODO

## Version 0.1.0 - MVP

- ✅ Watch multiple databases at a time
- ✅ Configurable via config files (one for config, one for database auth creds)
- ✅ Databases should have independent, configurable times for long queries
- ✅ Better use of contexts and signal handling
- ✅ More detailed logging when a query/txn is killed
- ✅ Improve test coverage
- Long transaction (txn) detection and killing
  - slog logs this currently, but we should include some extra identifiable info if we wanted to create monitors
  - We might also want to include the digest of said query, if we can strip out the PII
- Use the new [synthetic time](https://antonz.org/go-1-25/#synthetic-time-for-testing) in the tests
- Copy long query time from web into the settings
- See if the sniper can detect the `MYSQL_TIMEOUT` (or whatever it is) query hint and abide by that setting rather than the default

## Longer Term Features

- Post to Slack when a query/txn is detected (and/or killed)
  - This will require extra configuration, and should be entirely optional
- Fire a Pagerduty alert when a query/txn is detected (and/or killed)
  - This will also require extra configuration, and should be entirely optional
- Post something or other to Datadog when a query/txn is detected (and/or killed)
  - An event?
  - A metric?
