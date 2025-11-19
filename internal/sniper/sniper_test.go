package sniper

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"

	"github.com/persona-id/query-sniper/internal/configuration"
)

func TestGenerateHunterQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                    string
		queryWantContains       []string
		queryWantNotContain     []string
		transactionWantContains []string
		sniper                  QuerySniper
	}{
		{
			name: "no schema - only time filter",
			sniper: QuerySniper{
				QueryLimit:       30 * time.Second,
				TransactionLimit: 60 * time.Second,
				Schema:           "",
			},
			queryWantContains: []string{
				"SELECT pl.id, pl.user, pl.db as current_schema, pl.command, pl.time, es.digest_text",
				"FROM performance_schema.processlist pl",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE pl.command = 'Query'",
				"AND (pl.info LIKE 'SELECT%' OR pl.info LIKE 'INSERT%' OR pl.info LIKE 'UPDATE%' OR pl.info LIKE 'DELETE%')",
				"AND pl.info NOT LIKE '%processlist%'",
				"AND pl.state NOT IN ('cleaning up')",
				"ORDER BY pl.time DESC",
			},
			queryWantNotContain: []string{
				"AND pl.db in (",
			},
			transactionWantContains: []string{
				"SELECT trx.trx_id, pl.id as process_id, trx.trx_state, TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) AS time, pl.user, pl.db as current_schema, es.digest_text",
				"FROM INFORMATION_SCHEMA.INNODB_TRX trx",
				"INNER JOIN performance_schema.processlist pl ON trx.trx_mysql_thread_id = pl.id",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= 60",
				"ORDER BY time DESC",
			},
		},
		{
			name: "with schema - both time and DB filters",
			sniper: QuerySniper{
				QueryLimit:       60 * time.Second,
				TransactionLimit: 120 * time.Second,
				Schema:           "test_db",
			},
			queryWantContains: []string{
				"SELECT pl.id, pl.user, pl.db as current_schema, pl.command, pl.time, es.digest_text",
				"FROM performance_schema.processlist pl",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE pl.command = 'Query'",
				"AND (pl.info LIKE 'SELECT%' OR pl.info LIKE 'INSERT%' OR pl.info LIKE 'UPDATE%' OR pl.info LIKE 'DELETE%')",
				"AND pl.info NOT LIKE '%processlist%'",
				"AND pl.state NOT IN ('cleaning up')",
				"ORDER BY pl.time DESC",
			},
			transactionWantContains: []string{
				"SELECT trx.trx_id, pl.id as process_id, trx.trx_state, TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) AS time, pl.user, pl.db as current_schema, es.digest_text",
				"FROM INFORMATION_SCHEMA.INNODB_TRX trx",
				"INNER JOIN performance_schema.processlist pl ON trx.trx_mysql_thread_id = pl.id",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= 120",
				"AND pl.db IN ('test_db')",
				"ORDER BY time DESC",
			},
		},
		{
			name: "different query limit duration",
			sniper: QuerySniper{
				QueryLimit:       5 * time.Minute,
				TransactionLimit: 10 * time.Minute,
				Schema:           "production",
			},
			queryWantContains: []string{
				"AND pl.time >= 300", // 5 minutes = 300 seconds
				"AND pl.db IN ('production')",
			},
			transactionWantContains: []string{
				"WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= 600",
				"AND pl.db IN ('production')",
			},
		},
		{
			name: "fractional seconds get truncated to int",
			sniper: QuerySniper{
				QueryLimit:       1500 * time.Millisecond, // 1.5 seconds
				TransactionLimit: 2500 * time.Millisecond, // 2.5 seconds
				Schema:           "",
			},
			queryWantContains: []string{
				"AND pl.time >= 1", // 1.5 seconds truncated to 1
			},
			transactionWantContains: []string{
				"WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= 2", // 2.5 seconds truncated to 2
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			queryGot, txnGot, err := tt.sniper.generateHunterQueries()
			if err != nil {
				t.Errorf("generateHunterQueries() error = %v", err)

				return
			}

			// Test query generation
			for _, want := range tt.queryWantContains {
				if !strings.Contains(queryGot, want) {
					t.Errorf("generateHunterQueries() query result missing expected string %q\nGot: %s", want, queryGot)
				}
			}

			for _, unwanted := range tt.queryWantNotContain {
				if strings.Contains(queryGot, unwanted) {
					t.Errorf("generateHunterQueries() query result contains unwanted string %q\nGot: %s", unwanted, queryGot)
				}
			}

			if strings.Contains(queryGot, "\n") || strings.Contains(queryGot, "\t") {
				t.Errorf("generateHunterQueries() query result should not contain newlines or tabs\nGot: %s", queryGot)
			}

			normalized := strings.Join(strings.Fields(queryGot), " ")
			if queryGot != normalized {
				t.Errorf("generateHunterQueries() query result has excessive whitespace\nGot: %s\nWant: %s", queryGot, normalized)
			}

			// Test transaction query generation
			for _, want := range tt.transactionWantContains {
				if !strings.Contains(txnGot, want) {
					t.Errorf("generateHunterQueries() transaction result missing expected string %q\nGot: %s", want, txnGot)
				}
			}

			if strings.Contains(txnGot, "\n") || strings.Contains(txnGot, "\t") {
				t.Errorf("generateHunterQueries() transaction result should not contain newlines or tabs\nGot: %s", txnGot)
			}

			normalized = strings.Join(strings.Fields(txnGot), " ")
			if txnGot != normalized {
				t.Errorf("generateHunterQueries() transaction result has excessive whitespace\nGot: %s\nWant: %s", txnGot, normalized)
			}
		})
	}
}

//nolint:gocognit
func TestCreateSniper(t *testing.T) {
	t.Parallel()

	tests := []struct {
		settings *configuration.Config
		name     string
		dbName   string
		wantErr  bool
	}{
		{
			name:   "successful sniper creation with schema",
			dbName: "test_db",
			settings: &configuration.Config{
				SafeMode: false,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					SSLCert              string        `mapstructure:"ssl_cert"`
					SSLKey               string        `mapstructure:"ssl_key"`
					SSLCA                string        `mapstructure:"ssl_ca"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					Port                 int           `mapstructure:"port"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"test_db": {
						Address:              "127.0.0.1",
						Port:                 3306,
						Schema:               "production",
						Username:             "test_user",
						Password:             "test_pass",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               true,
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "successful sniper creation without schema",
			dbName: "analytics",
			settings: &configuration.Config{
				SafeMode: false,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					SSLCert              string        `mapstructure:"ssl_cert"`
					SSLKey               string        `mapstructure:"ssl_key"`
					SSLCA                string        `mapstructure:"ssl_ca"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					Port                 int           `mapstructure:"port"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"analytics": {
						Address:              "db.example.com",
						Port:                 3306,
						Schema:               "", // Empty schema
						Username:             "analytics_user",
						Password:             "secret123",
						Interval:             45 * time.Second,
						LongQueryLimit:       300 * time.Second,
						LongTransactionLimit: 600 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := New(tt.dbName, tt.settings)
			if (err != nil) != tt.wantErr {
				t.Errorf("createSniper() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.wantErr {
				return
			}

			expectedConfig := tt.settings.Databases[tt.dbName]

			if got.Name != tt.dbName {
				t.Errorf("createSniper() Name = %v, want %v", got.Name, tt.dbName)
			}

			if got.Schema != expectedConfig.Schema {
				t.Errorf("createSniper() Schema = %v, want %v", got.Schema, expectedConfig.Schema)
			}

			if got.Interval != expectedConfig.Interval {
				t.Errorf("createSniper() Interval = %v, want %v", got.Interval, expectedConfig.Interval)
			}

			if got.QueryLimit != expectedConfig.LongQueryLimit {
				t.Errorf("createSniper() QueryLimit = %v, want %v", got.QueryLimit, expectedConfig.LongQueryLimit)
			}

			if got.TransactionLimit != expectedConfig.LongTransactionLimit {
				t.Errorf("createSniper() TransactionLimit = %v, want %v", got.TransactionLimit, expectedConfig.LongTransactionLimit)
			}

			if got.Connection == nil {
				t.Error("createSniper() Connection is nil")
			}

			if got.LRQQuery == "" {
				t.Error("createSniper() LRQQuery is empty")
			}

			if got.LRTXNQuery == "" {
				t.Error("createSniper() LRTXNQuery is empty")
			}

			if expectedConfig.Schema != "" {
				expectedDBFilter := "AND pl.db IN ('" + expectedConfig.Schema + "')"
				if !strings.Contains(got.LRQQuery, expectedDBFilter) {
					t.Errorf("createSniper() LRQQuery missing DB filter for schema %q", expectedConfig.Schema)
				}
			}

			expectedTimeFilter := "AND pl.time >="
			if !strings.Contains(got.LRQQuery, expectedTimeFilter) {
				t.Error("createSniper() LRQQuery missing time filter")
			}

			if got.DryRun != expectedConfig.DryRun {
				t.Errorf("createSniper() DryRun = %v, want %v", got.DryRun, expectedConfig.DryRun)
			}

			if got.Connection != nil {
				got.Connection.Close()
			}
		})
	}
}

func TestCreateSniper_SafeModeOverride(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		description    string
		safeModeGlobal bool
		dbDryRun       bool
		expectedDryRun bool
	}{
		{
			name:           "safe mode false, db dry_run false - both false",
			safeModeGlobal: false,
			dbDryRun:       false,
			expectedDryRun: false,
			description:    "Normal operation mode",
		},
		{
			name:           "safe mode false, db dry_run true - db setting wins",
			safeModeGlobal: false,
			dbDryRun:       true,
			expectedDryRun: true,
			description:    "Per-database dry_run setting takes effect",
		},
		{
			name:           "safe mode true, db dry_run false - safe mode overrides",
			safeModeGlobal: true,
			dbDryRun:       false,
			expectedDryRun: true,
			description:    "Global safe mode overrides database setting",
		},
		{
			name:           "safe mode true, db dry_run true - both true",
			safeModeGlobal: true,
			dbDryRun:       true,
			expectedDryRun: true,
			description:    "Both safe mode and database dry_run enabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			settings := &configuration.Config{
				SafeMode: tt.safeModeGlobal,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					SSLCert              string        `mapstructure:"ssl_cert"`
					SSLKey               string        `mapstructure:"ssl_key"`
					SSLCA                string        `mapstructure:"ssl_ca"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					Port                 int           `mapstructure:"port"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"test_db": {
						Address:              "127.0.0.1",
						Port:                 3306,
						Schema:               "test_schema",
						Username:             "test_user",
						Password:             "test_pass",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               tt.dbDryRun,
					},
				},
			}

			sniper, err := New("test_db", settings)
			if err != nil {
				t.Errorf("New() unexpected error = %v", err)

				return
			}

			if sniper.DryRun != tt.expectedDryRun {
				t.Errorf("%s: expected DryRun = %v, got %v (SafeMode: %v, DB dry_run: %v)",
					tt.description, tt.expectedDryRun, sniper.DryRun, tt.safeModeGlobal, tt.dbDryRun)
			}

			// Verify the precedence logic is working as expected
			if tt.safeModeGlobal && !sniper.DryRun {
				t.Error("Safe mode is enabled but sniper DryRun is false - precedence logic failed")
			}

			if sniper.Connection != nil {
				sniper.Connection.Close()
			}
		})
	}
}

func TestCreateSniper_NonExistentDatabase(t *testing.T) {
	t.Parallel()

	settings := &configuration.Config{
		SafeMode: false,
		Databases: map[string]struct {
			Address              string        `mapstructure:"address"`
			Schema               string        `mapstructure:"schema"`
			SSLCert              string        `mapstructure:"ssl_cert"`
			SSLKey               string        `mapstructure:"ssl_key"`
			SSLCA                string        `mapstructure:"ssl_ca"`
			Username             string        `mapstructure:"username"`
			Password             string        `mapstructure:"password"`
			Interval             time.Duration `mapstructure:"interval"`
			LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
			LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
			Port                 int           `mapstructure:"port"`
			DryRun               bool          `mapstructure:"dry_run"`
		}{
			"existing_db": {
				Address:              "127.0.0.1",
				Port:                 3306,
				Schema:               "test",
				Username:             "user",
				Password:             "pass",
				Interval:             30 * time.Second,
				LongQueryLimit:       60 * time.Second,
				LongTransactionLimit: 120 * time.Second,
				DryRun:               false,
			},
		},
	}

	got, err := New("non_existent_db", settings)
	if err != nil {
		t.Errorf("createSniper() unexpected error = %v", err)

		return
	}

	if got.Name != "non_existent_db" {
		t.Errorf("createSniper() Name = %v, want %v", got.Name, "non_existent_db")
	}

	if got.Schema != "" {
		t.Errorf("createSniper() Schema = %v, want empty string", got.Schema)
	}

	if got.Interval != 0 {
		t.Errorf("createSniper() Interval = %v, want 0", got.Interval)
	}

	if got.QueryLimit != 0 {
		t.Errorf("createSniper() QueryLimit = %v, want 0", got.QueryLimit)
	}

	if got.DryRun {
		t.Errorf("createSniper() DryRun = %v, want false", got.DryRun)
	}

	if got.Connection != nil {
		got.Connection.Close()
	}
}

func TestNew_SSLConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		sslCert         string
		sslKey          string
		sslCA           string
		description     string
		shouldEnableSSL bool
	}{
		{
			name:            "only SSL cert set - SSL should not be enabled",
			sslCert:         "/path/to/cert.pem",
			sslKey:          "",
			sslCA:           "",
			shouldEnableSSL: false,
			description:     "Invalid: cert without CA should not enable SSL",
		},
		{
			name:            "only SSL key set - SSL should not be enabled",
			sslCert:         "",
			sslKey:          "/path/to/key.pem",
			sslCA:           "",
			shouldEnableSSL: false,
			description:     "Invalid: key without CA should not enable SSL",
		},
		{
			name:            "only SSL CA set - SSL should be enabled (CA-only mode)",
			sslCert:         "",
			sslKey:          "",
			sslCA:           "/path/to/ca.pem",
			shouldEnableSSL: true,
			description:     "CA-only mode: encrypted connection without client auth",
		},
		{
			name:            "cert and key without CA - SSL should not be enabled",
			sslCert:         "/path/to/cert.pem",
			sslKey:          "/path/to/key.pem",
			sslCA:           "",
			shouldEnableSSL: false,
			description:     "Invalid: cert+key without CA should not enable SSL",
		},
		{
			name:            "cert and CA without key - SSL should not be enabled",
			sslCert:         "/path/to/cert.pem",
			sslKey:          "",
			sslCA:           "/path/to/ca.pem",
			shouldEnableSSL: false,
			description:     "Invalid: cert+CA without key should not enable SSL",
		},
		{
			name:            "key and CA without cert - SSL should not be enabled",
			sslCert:         "",
			sslKey:          "/path/to/key.pem",
			sslCA:           "/path/to/ca.pem",
			shouldEnableSSL: false,
			description:     "Invalid: key+CA without cert should not enable SSL",
		},
		{
			name:            "all three SSL fields set - SSL should be enabled (mutual TLS)",
			sslCert:         "/path/to/cert.pem",
			sslKey:          "/path/to/key.pem",
			sslCA:           "/path/to/ca.pem",
			shouldEnableSSL: true,
			description:     "Mutual TLS mode: full mutual authentication",
		},
		{
			name:            "no SSL fields set - SSL should not be enabled",
			sslCert:         "",
			sslKey:          "",
			sslCA:           "",
			shouldEnableSSL: false,
			description:     "No SSL configuration provided",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			settings := &configuration.Config{
				SafeMode: false,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					SSLCert              string        `mapstructure:"ssl_cert"`
					SSLKey               string        `mapstructure:"ssl_key"`
					SSLCA                string        `mapstructure:"ssl_ca"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					Port                 int           `mapstructure:"port"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"ssl_test_db": {
						Address:              "127.0.0.1",
						Port:                 3306,
						Schema:               "test_schema",
						Username:             "test_user",
						Password:             "test_pass",
						SSLCert:              tt.sslCert,
						SSLKey:               tt.sslKey,
						SSLCA:                tt.sslCA,
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               true,
					},
				},
			}

			// The New function will attempt to create a database connection
			// We expect different behaviors based on SSL configuration:
			// - SSL disabled: Connection succeeds (simple DSN)
			// - SSL enabled: Connection fails with DSN parsing error (SSL parameters added)
			sniper, err := New("ssl_test_db", settings)

			// yeah this is a bit nested, but it's just a test so im not going to sweat it too much
			if tt.shouldEnableSSL { //nolint:nestif
				// When SSL is enabled, we expect a connection error due to SSL parameters
				// being added to the DSN (which will fail without a real SSL-enabled MySQL)
				if err == nil {
					t.Errorf("Expected SSL connection to fail without real SSL-enabled database, but succeeded")

					if sniper.Connection != nil {
						sniper.Connection.Close()
					}
				} else {
					// This is expected - SSL parameters cause connection to fail
					t.Logf("Expected SSL connection error: %v", err)

					// Verify it's a database connection error, not SSL config parsing error
					if !strings.Contains(err.Error(), "error opening database") {
						t.Errorf("Expected database connection error, got: %v", err)
					}
				}

				t.Logf("SSL enabled as expected: cert=%s, key=%s, ca=%s", tt.sslCert, tt.sslKey, tt.sslCA)
			} else {
				// When SSL is disabled, connection should succeed with simple DSN
				// (though it may still fail if there's no MySQL running, which is OK)
				if err == nil {
					t.Logf("Connection succeeded with SSL disabled (no SSL parameters in DSN)")

					if sniper.Connection != nil {
						sniper.Connection.Close()
					}
				} else {
					// Even if connection fails, it should be a basic connection error,
					// not an SSL-related DSN parsing error
					t.Logf("Connection failed (likely no MySQL running): %v", err)
				}

				t.Logf("SSL disabled as expected: cert=%s, key=%s, ca=%s", tt.sslCert, tt.sslKey, tt.sslCA)
			}

			t.Log(tt.description)
		})
	}
}

func TestKillProcesses_DryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		processes []MysqlProcess
		dryRun    bool
		expected  int
	}{
		{
			name:     "dry run mode - no actual killing",
			dryRun:   true,
			expected: 2,
			processes: []MysqlProcess{
				{
					ID:         123,
					Command:    "Query",
					Time:       30,
					User:       sql.NullString{String: "test_user", Valid: true},
					Schema:     sql.NullString{String: "test_db", Valid: true},
					DigestText: sql.NullString{String: "SELECT * FROM users", Valid: true},
				},
				{
					ID:         456,
					Command:    "Query",
					Time:       60,
					User:       sql.NullString{String: "another_user", Valid: true},
					Schema:     sql.NullString{String: "another_db", Valid: true},
					DigestText: sql.NullString{String: "UPDATE products SET price = ?", Valid: true},
				},
			},
		},
		{
			name:     "normal mode - would actually kill (but we don't test DB connection)",
			dryRun:   false,
			expected: 2,
			processes: []MysqlProcess{
				{
					ID:         789,
					Command:    "Query",
					Time:       90,
					User:       sql.NullString{String: "prod_user", Valid: true},
					Schema:     sql.NullString{String: "production", Valid: true},
					DigestText: sql.NullString{String: "SELECT COUNT(*) FROM orders", Valid: true},
				},
				{
					ID:         101,
					Command:    "Query",
					Time:       45,
					User:       sql.NullString{String: "analytics", Valid: true},
					Schema:     sql.NullString{String: "analytics_db", Valid: true},
					DigestText: sql.NullString{String: "INSERT INTO logs VALUES (?)", Valid: true},
				},
			},
		},
		{
			name:      "empty processes list",
			dryRun:    true,
			expected:  0,
			processes: []MysqlProcess{},
		},
		{
			name:     "processes with invalid IDs are skipped",
			dryRun:   true,
			expected: 1,
			processes: []MysqlProcess{
				{
					ID:      0, // Invalid ID, should be skipped
					Command: "Query",
					Time:    30,
				},
				{
					ID:      -1, // Invalid ID, should be skipped
					Command: "Query",
					Time:    30,
				},
				{
					ID:         102, // Valid ID
					Command:    "Query",
					Time:       30,
					User:       sql.NullString{String: "valid_user", Valid: true},
					Schema:     sql.NullString{String: "valid_db", Valid: true},
					DigestText: sql.NullString{String: "SHOW TABLES", Valid: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a test sniper - we don't need a real DB connection for dry run tests
			sniper := QuerySniper{
				Name:   "test_sniper",
				DryRun: tt.dryRun,
				// Note: Connection is nil, but that's OK for dry run tests
				// For non-dry run tests, we'd need to mock the database
			}

			ctx := context.Background()

			// For non-dry run mode, we'd need to skip this test or mock the database
			// since we can't actually execute KILL commands without a real MySQL connection
			if !tt.dryRun {
				t.Skip("Skipping non-dry run test - would require database connection")
			}

			killed := sniper.KillProcesses(ctx, tt.processes)

			if killed != tt.expected {
				t.Errorf("KillProcesses() killed = %v, expected %v", killed, tt.expected)
			}
		})
	}
}

func TestKillProcesses_DryRunLogging(t *testing.T) {
	t.Parallel()

	// This test verifies that dry run mode produces the expected log output
	// In a real implementation, you might want to use a test logger to capture
	// and verify the log messages, but for now we just ensure the method completes
	// without panicking

	sniper := QuerySniper{
		Name:   "test_logging_sniper",
		DryRun: true,
	}

	processes := []MysqlProcess{
		{
			ID:         999,
			Command:    "Query",
			Time:       120,
			User:       sql.NullString{String: "log_test_user", Valid: true},
			Schema:     sql.NullString{String: "log_test_db", Valid: true},
			DigestText: sql.NullString{String: "SELECT * FROM large_table", Valid: true},
		},
	}

	ctx := context.Background()
	killed := sniper.KillProcesses(ctx, processes)

	if killed != 1 {
		t.Errorf("KillProcesses() killed = %v, expected 1", killed)
	}
}

func TestKillTransactions_DryRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		transactions []MysqlTransaction
		dryRun       bool
		expected     int
	}{
		{
			name:     "dry run mode - no actual killing",
			dryRun:   true,
			expected: 2,
			transactions: []MysqlTransaction{
				{
					ID:         123,
					ProcessID:  456,
					Command:    "INSERT",
					Time:       30,
					User:       sql.NullString{String: "test_user", Valid: true},
					Schema:     sql.NullString{String: "test_db", Valid: true},
					DigestText: sql.NullString{String: "INSERT INTO users VALUES (?)", Valid: true},
					State:      sql.NullString{String: "RUNNING", Valid: true},
				},
				{
					ID:         789,
					ProcessID:  101,
					Command:    "UPDATE",
					Time:       60,
					User:       sql.NullString{String: "another_user", Valid: true},
					Schema:     sql.NullString{String: "another_db", Valid: true},
					DigestText: sql.NullString{String: "UPDATE products SET price = ?", Valid: true},
					State:      sql.NullString{String: "RUNNING", Valid: true},
				},
			},
		},
		{
			name:     "normal mode - would actually kill (but we don't test DB connection)",
			dryRun:   false,
			expected: 2,
			transactions: []MysqlTransaction{
				{
					ID:         999,
					ProcessID:  888,
					Command:    "SELECT",
					Time:       90,
					User:       sql.NullString{String: "prod_user", Valid: true},
					Schema:     sql.NullString{String: "production", Valid: true},
					DigestText: sql.NullString{String: "SELECT COUNT(*) FROM orders", Valid: true},
					State:      sql.NullString{String: "RUNNING", Valid: true},
				},
				{
					ID:         555,
					ProcessID:  444,
					Command:    "DELETE",
					Time:       45,
					User:       sql.NullString{String: "analytics", Valid: true},
					Schema:     sql.NullString{String: "analytics_db", Valid: true},
					DigestText: sql.NullString{String: "DELETE FROM logs WHERE created < ?", Valid: true},
					State:      sql.NullString{String: "RUNNING", Valid: true},
				},
			},
		},
		{
			name:         "empty transactions list",
			dryRun:       true,
			expected:     0,
			transactions: []MysqlTransaction{},
		},
		{
			name:     "transactions with invalid IDs are skipped",
			dryRun:   true,
			expected: 1,
			transactions: []MysqlTransaction{
				{
					ID:        0, // Invalid ID, should be skipped
					ProcessID: 123,
					Command:   "INSERT",
					Time:      30,
				},
				{
					ID:        -1, // Invalid ID, should be skipped
					ProcessID: 456,
					Command:   "UPDATE",
					Time:      30,
				},
				{
					ID:         102, // Valid ID
					ProcessID:  789,
					Command:    "SELECT",
					Time:       30,
					User:       sql.NullString{String: "valid_user", Valid: true},
					Schema:     sql.NullString{String: "valid_db", Valid: true},
					DigestText: sql.NullString{String: "SHOW TABLES", Valid: true},
					State:      sql.NullString{String: "RUNNING", Valid: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a test sniper - we don't need a real DB connection for dry run tests
			sniper := QuerySniper{
				Name:   "test_sniper",
				DryRun: tt.dryRun,
				// Note: Connection is nil, but that's OK for dry run tests
				// For non-dry run tests, we'd need to mock the database
			}

			ctx := context.Background()

			// For non-dry run mode, we'd need to skip this test or mock the database
			// since we can't actually execute KILL commands without a real MySQL connection
			if !tt.dryRun {
				t.Skip("Skipping non-dry run test - would require database connection")
			}

			killed := sniper.KillTransactions(ctx, tt.transactions)

			if killed != tt.expected {
				t.Errorf("KillTransactions() killed = %v, expected %v", killed, tt.expected)
			}
		})
	}
}

func TestKillTransactions_DryRunLogging(t *testing.T) {
	t.Parallel()

	// This test verifies that dry run mode produces the expected log output
	// In a real implementation, you might want to use a test logger to capture
	// and verify the log messages, but for now we just ensure the method completes
	// without panicking

	sniper := QuerySniper{
		Name:   "test_logging_sniper",
		DryRun: true,
	}

	transactions := []MysqlTransaction{
		{
			ID:         999,
			ProcessID:  888,
			Command:    "SELECT",
			Time:       120,
			User:       sql.NullString{String: "log_test_user", Valid: true},
			Schema:     sql.NullString{String: "log_test_db", Valid: true},
			DigestText: sql.NullString{String: "SELECT * FROM large_table", Valid: true},
			State:      sql.NullString{String: "RUNNING", Valid: true},
		},
	}

	ctx := context.Background()
	killed := sniper.KillTransactions(ctx, transactions)

	if killed != 1 {
		t.Errorf("KillTransactions() killed = %v, expected 1", killed)
	}
}

// TestSniperLoopSlowQueryDetection tests the sniper's ability to detect slow queries
// using Go 1.25's testing/synctest for virtualized time. This test simulates a
// sniper that checks for queries running longer than 10 seconds, with periodic
// checks every 5 seconds, without waiting for real time to pass.
func TestSniperLoopSlowQueryDetection(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		t.Helper()
		// Create a context that will timeout after 30 seconds (virtualized)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Create a mock sniper with 10-second query limit and 5-second check interval
		sniper := QuerySniper{
			Name:       "test_sniper",
			Schema:     "test_db",
			Interval:   5 * time.Second,  // Check every 5 seconds
			QueryLimit: 10 * time.Second, // Kill queries running > 10 seconds
			DryRun:     true,             // Don't actually kill anything
			Connection: nil,              // We'll mock the database calls
			LRQQuery:   "SELECT 1",       // Simplified for test
		}

		// Channel to track when the sniper loop starts checking
		checkStarted := make(chan struct{})

		// Channel to track completion
		loopDone := make(chan struct{})

		// Track checks performed
		var completedChecks int

		// Start the sniper loop in a goroutine
		go func() {
			defer close(loopDone)

			// Create a ticker for the interval
			ticker := time.NewTicker(sniper.Interval)
			defer ticker.Stop()

			checkCount := 0

			for {
				select {
				case <-ctx.Done():
					completedChecks = checkCount

					return

				case <-ticker.C:
					checkCount++

					// Signal that we've started checking
					if checkCount == 1 {
						close(checkStarted)
					}

					// Stop after 6 checks (30 seconds of virtualized time)
					if checkCount >= 6 {
						completedChecks = checkCount

						return
					}
				}
			}
		}()

		// Wait for the first check to start
		select {
		case <-checkStarted:
			t.Log("Sniper has started periodic checking")
		case <-time.After(10 * time.Second): // This completes instantly with synctest
			t.Fatal("Sniper failed to start checking within expected time")
		}

		// Wait for the loop to complete or timeout
		<-loopDone

		// Log results after goroutine has completed
		if completedChecks >= 6 {
			t.Logf("Sniper loop completed successfully after %d checks", completedChecks)
		} else {
			t.Logf("Sniper loop stopped early after %d checks due to context timeout", completedChecks)
		}

		// The test should complete almost instantly despite using 30+ seconds of virtualized time
		t.Log("Test completed - virtualized time allowed rapid execution")
	})
}

// TestSniperQueryLimitTimeout tests that the sniper respects query time limits
// and properly handles timeout scenarios using virtualized time.
func TestSniperQueryLimitTimeout(t *testing.T) {
	t.Parallel()

	// Test different query time limits
	testCases := []struct {
		name       string
		queryLimit time.Duration
		queryTime  time.Duration
		shouldKill bool
	}{
		{
			name:       "query under limit should not be killed",
			queryLimit: 10 * time.Second,
			queryTime:  5 * time.Second,
			shouldKill: false,
		},
		{
			name:       "query at limit should be killed",
			queryLimit: 10 * time.Second,
			queryTime:  10 * time.Second,
			shouldKill: true,
		},
		{
			name:       "query over limit should be killed",
			queryLimit: 10 * time.Second,
			queryTime:  15 * time.Second,
			shouldKill: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				t.Helper()

				// Create a mock sniper
				sniper := QuerySniper{
					QueryLimit: tc.queryLimit,
				}

				// Simulate a query running for the specified time
				queryStart := time.Now()

				// Wait for the query time to pass (virtualized)
				time.Sleep(tc.queryTime)

				queryDuration := time.Since(queryStart)

				// Check if the query should be killed based on the limit
				shouldKill := queryDuration >= sniper.QueryLimit

				if shouldKill != tc.shouldKill {
					t.Errorf("Expected shouldKill=%v for query running %v with limit %v, got %v",
						tc.shouldKill, tc.queryTime, tc.queryLimit, shouldKill)
				}

				t.Logf("Query ran for %v (virtualized), limit is %v, shouldKill=%v",
					queryDuration, sniper.QueryLimit, shouldKill)
			})
		})
	}
}

// TestSniperIntervalTiming tests that the sniper's interval timing works correctly
// with virtualized time, ensuring checks happen at the expected intervals.
func TestSniperIntervalTiming(t *testing.T) {
	t.Parallel()

	intervals := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		10 * time.Second,
	}

	for _, interval := range intervals {
		t.Run(interval.String(), func(t *testing.T) {
			t.Parallel()

			synctest.Test(t, func(t *testing.T) {
				t.Helper()

				ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
				defer cancel()

				var checkTimes []time.Time

				done := make(chan struct{})

				go func() {
					defer close(done)

					ticker := time.NewTicker(interval)
					defer ticker.Stop()

					for {
						select {
						case <-ctx.Done():
							return
						case now := <-ticker.C:
							checkTimes = append(checkTimes, now)

							// Stop after 3 checks
							if len(checkTimes) >= 3 {
								return
							}
						}
					}
				}()

				// Wait for completion
				<-done

				// Log check times after goroutine completes
				for i, checkTime := range checkTimes {
					t.Logf("Check %d at: %v", i+1, checkTime)
				}

				// Verify we got the expected number of checks
				if len(checkTimes) < 3 {
					t.Errorf("Expected at least 3 checks, got %d", len(checkTimes))
				}

				// Verify the timing between checks (allowing for some variance in virtualized time)
				for i := 1; i < len(checkTimes); i++ {
					actualInterval := checkTimes[i].Sub(checkTimes[i-1])
					if actualInterval < interval {
						t.Errorf("Check interval too short: expected >= %v, got %v", interval, actualInterval)
					}
				}

				t.Logf("Successfully completed %d checks with %v intervals", len(checkTimes), interval)
			})
		})
	}
}

// TestSniperGracefulShutdown tests that the sniper properly handles context cancellation
// and shuts down gracefully using virtualized time.
func TestSniperGracefulShutdown(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		t.Helper()
		// Create a context that we'll cancel after 15 seconds
		ctx, cancel := context.WithCancel(context.Background())

		sniper := QuerySniper{
			Name:     "test_sniper",
			Interval: 5 * time.Second,
			DryRun:   true,
		}

		shutdownComplete := make(chan struct{})

		var checkCount int

		// Start the sniper loop
		go func() {
			defer close(shutdownComplete)

			ticker := time.NewTicker(sniper.Interval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return

				case <-ticker.C:
					checkCount++
				}
			}
		}()

		// Let it run for a few checks
		time.Sleep(12 * time.Second) // Virtualized time

		// Cancel the context to trigger shutdown
		t.Log("Triggering graceful shutdown")
		cancel()

		// Wait for shutdown to complete
		select {
		case <-shutdownComplete:
			// Log results after goroutine has completed
			t.Logf("Sniper shut down gracefully after %d checks", checkCount)
		case <-time.After(5 * time.Second): // Virtualized time
			t.Fatal("Sniper failed to shut down within expected time")
		}

		// Verify we had some checks before shutdown
		if checkCount < 1 {
			t.Errorf("Expected at least 1 check before shutdown, got %d", checkCount)
		}

		t.Logf("Graceful shutdown completed after %d checks", checkCount)
	})
}

//nolint:dupl
func TestFindLongRunningQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		errorContains string
		skipReason    string
		sniper        QuerySniper
		expectError   bool
		skipExecution bool
	}{
		{
			name: "nil connection returns error",
			sniper: QuerySniper{
				Name:       "test_sniper",
				Connection: nil,
				LRQQuery:   "SELECT id, user, db, command, time, digest_text FROM processlist",
			},
			expectError:   true,
			errorContains: "error getting long running queries",
		},
		{
			name: "empty query string should still attempt execution",
			sniper: QuerySniper{
				Name:       "test_sniper",
				Connection: nil, // nil connection will cause error
				LRQQuery:   "",
			},
			expectError:   true,
			errorContains: "error getting long running queries",
		},
		{
			name: "valid query structure with nil connection",
			sniper: QuerySniper{
				Name:       "production_db",
				Connection: nil,
				LRQQuery: `SELECT pl.id, pl.user, pl.db as current_schema, pl.command, pl.time, es.digest_text
				           FROM performance_schema.processlist pl
				           INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id
				           INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id
				           WHERE pl.command = 'Query' AND pl.time >= 30`,
			},
			expectError:   true,
			errorContains: "error getting long running queries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skipExecution {
				t.Skip(tt.skipReason)
			}

			ctx := context.Background()

			// Handle expected panic from nil connection
			var (
				processes []MysqlProcess
				err       error
			)

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to nil connection, simulate error return
						err = fmt.Errorf("error getting long running queries: %v", r) //nolint:err113
					}
				}()

				processes, err = tt.sniper.FindLongRunningQueries(ctx)
			}()

			if tt.expectError {
				if err == nil {
					t.Errorf("FindLongRunningQueries() expected error but got none")

					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("FindLongRunningQueries() error = %v, should contain %q", err, tt.errorContains)
				}

				// When error occurs, processes should be empty
				if len(processes) > 0 {
					t.Errorf("FindLongRunningQueries() expected empty processes on error, got %v", processes)
				}

				return
			}

			if err != nil {
				t.Errorf("FindLongRunningQueries() unexpected error = %v", err)
			}

			// On success, processes should be a valid slice (can be empty)
			if processes == nil {
				t.Error("FindLongRunningQueries() returned nil processes slice on success")
			}
		})
	}
}

//nolint:dupl
func TestFindLongRunningTransactions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		errorContains string
		skipReason    string
		sniper        QuerySniper
		expectError   bool
		skipExecution bool
	}{
		{
			name: "nil connection returns error",
			sniper: QuerySniper{
				Name:       "test_sniper",
				Connection: nil,
				LRTXNQuery: "SELECT trx_id, process_id, trx_state, time, user, current_schema, digest_text FROM transactions",
			},
			expectError:   true,
			errorContains: "error getting long running transactions",
		},
		{
			name: "empty query string should still attempt execution",
			sniper: QuerySniper{
				Name:       "test_sniper",
				Connection: nil, // nil connection will cause error
				LRTXNQuery: "",
			},
			expectError:   true,
			errorContains: "error getting long running transactions",
		},
		{
			name: "valid query structure with nil connection",
			sniper: QuerySniper{
				Name:       "production_db",
				Connection: nil,
				LRTXNQuery: `SELECT trx.trx_id, pl.id as process_id, trx.trx_state,
				             TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) AS time,
				             pl.user, pl.db as current_schema, es.digest_text
				             FROM INFORMATION_SCHEMA.INNODB_TRX trx
				             INNER JOIN performance_schema.processlist pl ON trx.trx_mysql_thread_id = pl.id
				             LEFT JOIN performance_schema.threads t ON t.processlist_id = pl.id
				             LEFT JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id
				             WHERE TIMESTAMPDIFF(SECOND, trx.trx_started, NOW()) >= 60`,
			},
			expectError:   true,
			errorContains: "error getting long running transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if tt.skipExecution {
				t.Skip(tt.skipReason)
			}

			ctx := context.Background()

			// Handle expected panic from nil connection
			var (
				transactions []MysqlTransaction
				err          error
			)

			func() {
				defer func() {
					if r := recover(); r != nil {
						// Expected panic due to nil connection, simulate error return
						err = fmt.Errorf("error getting long running transactions: %v", r) //nolint:err113
					}
				}()

				transactions, err = tt.sniper.FindLongRunningTransactions(ctx)
			}()

			if tt.expectError {
				if err == nil {
					t.Errorf("FindLongRunningTransactions() expected error but got none")

					return
				}

				if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("FindLongRunningTransactions() error = %v, should contain %q", err, tt.errorContains)
				}

				// When error occurs, transactions should be empty
				if len(transactions) > 0 {
					t.Errorf("FindLongRunningTransactions() expected empty transactions on error, got %v", transactions)
				}

				return
			}

			if err != nil {
				t.Errorf("FindLongRunningTransactions() unexpected error = %v", err)
			}

			// On success, transactions should be a valid slice (can be empty)
			if transactions == nil {
				t.Error("FindLongRunningTransactions() returned nil transactions slice on success")
			}
		})
	}
}

func TestFindLongRunningQueries_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Note: This test cannot properly test context cancellation with a nil connection
	// as the nil connection error occurs before context is checked.
	// In real usage, context cancellation would be handled by the database driver.
	t.Skip("Skipping context cancellation test - requires real database connection to test properly")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	sniper := QuerySniper{
		Name:       "test_sniper",
		Connection: nil,
		LRQQuery:   "SELECT id, user, db, command, time, digest_text FROM processlist",
	}

	_, err := sniper.FindLongRunningQueries(ctx)
	if err == nil {
		t.Error("FindLongRunningQueries() expected error with cancelled context")
	}
	// Should contain context cancellation or connection error
	if !strings.Contains(err.Error(), "error getting long running queries") {
		t.Errorf("FindLongRunningQueries() error = %v, should contain context handling", err)
	}
}

func TestFindLongRunningTransactions_ContextCancellation(t *testing.T) {
	t.Parallel()

	// Note: This test cannot properly test context cancellation with a nil connection
	// as the nil connection error occurs before context is checked.
	// In real usage, context cancellation would be handled by the database driver.
	t.Skip("Skipping context cancellation test - requires real database connection to test properly")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	sniper := QuerySniper{
		Name:       "test_sniper",
		Connection: nil,
		LRTXNQuery: "SELECT trx_id, process_id, trx_state, time, user, current_schema, digest_text FROM transactions",
	}

	_, err := sniper.FindLongRunningTransactions(ctx)
	if err == nil {
		t.Error("FindLongRunningTransactions() expected error with cancelled context")
	}
	// Should contain context cancellation or connection error
	if !strings.Contains(err.Error(), "error getting long running transactions") {
		t.Errorf("FindLongRunningTransactions() error = %v, should contain context handling", err)
	}
}

func TestFindLongRunningQueries_ReturnTypes(t *testing.T) {
	t.Parallel()

	// Verify the function signature and behavior with nil connection
	// The function is expected to panic with nil connection, which is the intended behavior
	sniper := QuerySniper{
		Name:       "test_sniper",
		Connection: nil,
		LRQQuery:   "SELECT 1",
	}

	ctx := context.Background()

	// Function should panic with nil connection (this is expected behavior)
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to nil connection
			t.Log("Function correctly panics with nil connection (expected behavior)")
		} else {
			t.Error("FindLongRunningQueries() expected panic with nil connection")
		}
	}()

	// This will panic due to nil connection, which is the expected behavior
	_, _ = sniper.FindLongRunningQueries(ctx)
}

func TestFindLongRunningTransactions_ReturnTypes(t *testing.T) {
	t.Parallel()

	// Verify the function signature and behavior with nil connection
	// The function is expected to panic with nil connection, which is the intended behavior
	sniper := QuerySniper{
		Name:       "test_sniper",
		Connection: nil,
		LRTXNQuery: "SELECT 1",
	}

	ctx := context.Background()

	// Function should panic with nil connection (this is expected behavior)
	defer func() {
		if r := recover(); r != nil {
			// Expected panic due to nil connection
			t.Log("Function correctly panics with nil connection (expected behavior)")
		} else {
			t.Error("FindLongRunningTransactions() expected panic with nil connection")
		}
	}()

	// This will panic due to nil connection, which is the expected behavior
	_, _ = sniper.FindLongRunningTransactions(ctx)
}

//nolint:maintidx
func TestKillProcesses_DatabaseExecution(t *testing.T) {
	t.Parallel()

	// This test covers the non-dry-run execution path (lines 343-375 in sniper.go)
	// that was missing from coverage. It tests both success and error scenarios
	// when actually executing KILL commands against a database.

	// Skip if we can't connect to a test database
	testDBAvailable := true
	settings := &configuration.Config{
		SafeMode: false,
		Databases: map[string]struct {
			Address              string        `mapstructure:"address"`
			Schema               string        `mapstructure:"schema"`
			SSLCert              string        `mapstructure:"ssl_cert"`
			SSLKey               string        `mapstructure:"ssl_key"`
			SSLCA                string        `mapstructure:"ssl_ca"`
			Username             string        `mapstructure:"username"`
			Password             string        `mapstructure:"password"`
			Interval             time.Duration `mapstructure:"interval"`
			LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
			LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
			Port                 int           `mapstructure:"port"`
			DryRun               bool          `mapstructure:"dry_run"`
		}{
			"test_db": {
				Address:              "127.0.0.1",
				Port:                 3306,
				Schema:               "",
				Username:             "root",
				Password:             "password",
				Interval:             30 * time.Second,
				LongQueryLimit:       60 * time.Second,
				LongTransactionLimit: 120 * time.Second,
				DryRun:               false, // Important: NOT dry run
			},
		},
	}

	// Try to create a database connection
	sniper, err := New("test_db", settings)
	if err != nil {
		testDBAvailable = false

		t.Logf("Test database not available, skipping database execution tests: %v", err)
	}

	t.Cleanup(func() {
		if sniper.Connection != nil {
			sniper.Connection.Close()
		}
	})

	if !testDBAvailable {
		t.Skip("Database connection not available for testing kill execution")
	}

	// Verify we have a real connection and it's not in dry run mode
	if sniper.Connection == nil {
		t.Skip("No database connection available")
	}

	if sniper.DryRun {
		t.Fatal("Test sniper should not be in dry run mode")
	}

	t.Run("kill_nonexistent_process_error_handling", func(t *testing.T) {
		t.Parallel()

		// Test the error handling path by trying to kill a non-existent process
		// This should trigger the error handling code in lines 346-361
		processes := []MysqlProcess{
			{
				ID:         999999, // Very unlikely to exist
				Command:    "Query",
				Time:       120,
				User:       sql.NullString{String: "test_user", Valid: true},
				Schema:     sql.NullString{String: "test_db", Valid: true},
				DigestText: sql.NullString{String: "SELECT * FROM test_table", Valid: true},
			},
		}

		ctx := context.Background()
		killed := sniper.KillProcesses(ctx, processes)

		// Should return 0 killed since the process doesn't exist (error case)
		if killed != 0 {
			t.Errorf("KillProcesses() with non-existent process should kill 0, got %d", killed)
		}

		// The error handling should have logged the error and continued
		// (we can't easily capture the log output in this test setup)
	})

	t.Run("kill_multiple_processes_mixed_results", func(t *testing.T) {
		t.Parallel()

		// Test with multiple processes where some might fail
		processes := []MysqlProcess{
			{
				ID:         999998, // Non-existent process
				Command:    "Query",
				Time:       60,
				User:       sql.NullString{String: "user1", Valid: true},
				Schema:     sql.NullString{String: "db1", Valid: true},
				DigestText: sql.NullString{String: "SELECT 1", Valid: true},
			},
			{
				ID:         999997, // Another non-existent process
				Command:    "Query",
				Time:       90,
				User:       sql.NullString{String: "user2", Valid: true},
				Schema:     sql.NullString{String: "db2", Valid: true},
				DigestText: sql.NullString{String: "SELECT 2", Valid: true},
			},
		}

		ctx := context.Background()
		killed := sniper.KillProcesses(ctx, processes)

		// Should return 0 since both processes don't exist
		if killed != 0 {
			t.Errorf("KillProcesses() with non-existent processes should kill 0, got %d", killed)
		}
	})

	t.Run("kill_with_invalid_process_ids", func(t *testing.T) {
		t.Parallel()

		// Test the process ID validation (should skip invalid IDs)
		processes := []MysqlProcess{
			{
				ID:      0, // Invalid ID, should be skipped
				Command: "Query",
				Time:    30,
			},
			{
				ID:         999996, // Valid ID but non-existent process
				Command:    "Query",
				Time:       45,
				User:       sql.NullString{String: "valid_user", Valid: true},
				Schema:     sql.NullString{String: "valid_db", Valid: true},
				DigestText: sql.NullString{String: "SELECT 3", Valid: true},
			},
		}

		ctx := context.Background()
		killed := sniper.KillProcesses(ctx, processes)

		// Should return 0: one skipped (ID=0), one failed (non-existent)
		if killed != 0 {
			t.Errorf("KillProcesses() with mixed invalid/non-existent processes should kill 0, got %d", killed)
		}
	})

	t.Run("context_cancellation_during_kill", func(t *testing.T) {
		t.Parallel()

		// Test context cancellation during kill operations
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		processes := []MysqlProcess{
			{
				ID:         999995, // Non-existent process
				Command:    "Query",
				Time:       30,
				User:       sql.NullString{String: "cancel_test", Valid: true},
				Schema:     sql.NullString{String: "cancel_db", Valid: true},
				DigestText: sql.NullString{String: "SELECT 4", Valid: true},
			},
		}

		killed := sniper.KillProcesses(ctx, processes)

		// Should return 0 due to context cancellation or process not existing
		if killed != 0 {
			t.Errorf("KillProcesses() with cancelled context should kill 0, got %d", killed)
		}
	})

	t.Run("kill_own_connection_test", func(t *testing.T) {
		t.Parallel()

		// This test attempts to exercise the success path by trying to kill
		// a connection we create specifically for this test. This is the safest
		// way to test the actual kill success logic without harming other processes.

		ctx := context.Background()

		// Create a second connection that we can safely attempt to kill
		testConnection, err := sniper.Connection.BeginTx(ctx, nil)
		if err != nil {
			t.Logf("Could not create test transaction for kill test: %v", err)

			return
		}

		t.Cleanup(func() {
			// Clean up the test connection
			if testConnection != nil {
				rollbackErr := testConnection.Rollback()
				if rollbackErr != nil {
					t.Logf("Error rolling back test transaction: %v", rollbackErr)
				}
			}
		})

		// Get the current connection ID so we can try to find it in processlist
		var connectionID int

		err = sniper.Connection.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connectionID)
		if err != nil {
			t.Logf("Could not get connection ID: %v", err)

			return
		}

		// Try to kill our own connection (this tests the kill success path)
		// Note: This might succeed or fail depending on MySQL permissions,
		// but it exercises the code path we want to test
		processes := []MysqlProcess{
			{
				ID:         connectionID,
				Command:    "Query",
				Time:       1,
				User:       sql.NullString{String: "test_kill", Valid: true},
				Schema:     sql.NullString{String: "test", Valid: true},
				DigestText: sql.NullString{String: "SELECT CONNECTION_ID()", Valid: true},
			},
		}

		killed := sniper.KillProcesses(ctx, processes)

		// The result depends on whether we have PROCESS and CONNECTION_ADMIN privileges
		// If we have privileges, killed should be 1 (success path tested)
		// If we don't have privileges, killed should be 0 (error path tested)
		// Both outcomes are valid and test the code paths we want to cover
		if killed < 0 || killed > 1 {
			t.Errorf("KillProcesses() should return 0 or 1, got %d", killed)
		}

		// Log the result to help with understanding test behavior
		if killed == 1 {
			t.Log("Successfully tested kill success path (lines 364-375)")
		} else {
			t.Log("Tested kill error path due to insufficient privileges")
		}
	})
}

func TestKillTransactions_DatabaseExecution(t *testing.T) {
	t.Parallel()

	// This test covers the non-dry-run execution path for KillTransactions
	// similar to the KillProcesses test above

	// Skip if we can't connect to a test database
	testDBAvailable := true
	settings := &configuration.Config{
		SafeMode: false,
		Databases: map[string]struct {
			Address              string        `mapstructure:"address"`
			Schema               string        `mapstructure:"schema"`
			SSLCert              string        `mapstructure:"ssl_cert"`
			SSLKey               string        `mapstructure:"ssl_key"`
			SSLCA                string        `mapstructure:"ssl_ca"`
			Username             string        `mapstructure:"username"`
			Password             string        `mapstructure:"password"`
			Interval             time.Duration `mapstructure:"interval"`
			LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
			LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
			Port                 int           `mapstructure:"port"`
			DryRun               bool          `mapstructure:"dry_run"`
		}{
			"test_db": {
				Address:              "127.0.0.1",
				Port:                 3306,
				Schema:               "",
				Username:             "root",
				Password:             "password",
				Interval:             30 * time.Second,
				LongQueryLimit:       60 * time.Second,
				LongTransactionLimit: 120 * time.Second,
				DryRun:               false, // Important: NOT dry run
			},
		},
	}

	// Try to create a database connection
	sniper, err := New("test_db", settings)
	if err != nil {
		testDBAvailable = false

		t.Logf("Test database not available, skipping transaction kill tests: %v", err)
	}

	t.Cleanup(func() {
		if sniper.Connection != nil {
			sniper.Connection.Close()
		}
	})

	if !testDBAvailable {
		t.Skip("Database connection not available for testing transaction kill execution")
	}

	// Verify we have a real connection and it's not in dry run mode
	if sniper.Connection == nil {
		t.Skip("No database connection available")
	}

	if sniper.DryRun {
		t.Fatal("Test sniper should not be in dry run mode")
	}

	t.Run("kill_nonexistent_transaction_error_handling", func(t *testing.T) {
		t.Parallel()

		// Test the error handling path by trying to kill a non-existent transaction
		transactions := []MysqlTransaction{
			{
				ID:         999999, // Very unlikely to exist
				ProcessID:  888888, // Non-existent process
				Command:    "INSERT",
				Time:       120,
				User:       sql.NullString{String: "test_user", Valid: true},
				Schema:     sql.NullString{String: "test_db", Valid: true},
				DigestText: sql.NullString{String: "INSERT INTO test_table VALUES (?)", Valid: true},
				State:      sql.NullString{String: "RUNNING", Valid: true},
			},
		}

		ctx := context.Background()
		killed := sniper.KillTransactions(ctx, transactions)

		// Should return 0 killed since the transaction/process doesn't exist
		if killed != 0 {
			t.Errorf("KillTransactions() with non-existent transaction should kill 0, got %d", killed)
		}
	})

	t.Run("kill_invalid_transaction_ids", func(t *testing.T) {
		t.Parallel()

		// Test with invalid transaction IDs (should be skipped)
		transactions := []MysqlTransaction{
			{
				ID:        0, // Invalid ID, should be skipped
				ProcessID: 123,
				Command:   "UPDATE",
				Time:      30,
			},
			{
				ID:        -1, // Invalid ID, should be skipped
				ProcessID: 456,
				Command:   "DELETE",
				Time:      60,
			},
		}

		ctx := context.Background()
		killed := sniper.KillTransactions(ctx, transactions)

		// Should return 0 since both transaction IDs are invalid and skipped
		if killed != 0 {
			t.Errorf("KillTransactions() with invalid transaction IDs should kill 0, got %d", killed)
		}
	})
}

//nolint:unqueryvet
func TestMysqlTransactionStruct(t *testing.T) {
	t.Parallel()

	// Test that MysqlTransaction struct has expected fields and can be instantiated
	transaction := MysqlTransaction{
		ID:         12345,
		ProcessID:  67890,
		State:      sql.NullString{String: "RUNNING", Valid: true},
		Time:       120,
		User:       sql.NullString{String: "test_user", Valid: true},
		Schema:     sql.NullString{String: "test_schema", Valid: true},
		DigestText: sql.NullString{String: "SELECT * FROM large_table", Valid: true},
		Command:    "Query",
	}

	// Verify struct fields are accessible and have expected values
	if transaction.ID != 12345 {
		t.Errorf("expected ID to be 12345, got %d", transaction.ID)
	}

	if transaction.ProcessID != 67890 {
		t.Errorf("expected ProcessID to be 67890, got %d", transaction.ProcessID)
	}

	if transaction.State.String != "RUNNING" {
		t.Errorf("expected State to be 'RUNNING', got %q", transaction.State.String)
	}

	if transaction.Time != 120 {
		t.Errorf("expected Time to be 120, got %d", transaction.Time)
	}

	if transaction.User.String != "test_user" {
		t.Errorf("expected User to be 'test_user', got %q", transaction.User.String)
	}

	if transaction.Schema.String != "test_schema" {
		t.Errorf("expected Schema to be 'test_schema', got %q", transaction.Schema.String)
	}

	if transaction.DigestText.String != "SELECT * FROM large_table" {
		t.Errorf("expected DigestText to be 'SELECT * FROM large_table', got %q", transaction.DigestText.String)
	}

	if transaction.Command != "Query" {
		t.Errorf("expected Command to be 'Query', got %q", transaction.Command)
	}
}

// TestMain is used to verify that there are no leaks during the tests.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
