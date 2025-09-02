package sniper

import (
	"strings"
	"testing"
	"time"

	"github.com/persona-id/query-sniper/internal/configuration"
)

func TestGenerateHunterQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		wantContains   []string
		wantNotContain []string
		sniper         QuerySniper
	}{
		{
			name: "no schema - only time filter",
			sniper: QuerySniper{
				QueryLimit: 30 * time.Second,
				Schema:     "",
			},
			wantContains: []string{
				"SELECT pl.id, pl.user, pl.host, pl.db, pl.command, pl.time, es.digest_text, es.current_schema",
				"FROM performance_schema.processlist pl",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE pl.command NOT IN ('sleep', 'killed')",
				"AND pl.info NOT LIKE '%processlist%'",
				"AND pl.time >= 30",
				"ORDER BY pl.time DESC",
			},
			wantNotContain: []string{
				"AND pl.db in (",
			},
		},
		{
			name: "with schema - both time and DB filters",
			sniper: QuerySniper{
				QueryLimit: 60 * time.Second,
				Schema:     "test_db",
			},
			wantContains: []string{
				"SELECT pl.id, pl.user, pl.host, pl.db, pl.command, pl.time, es.digest_text, es.current_schema",
				"FROM performance_schema.processlist pl",
				"INNER JOIN performance_schema.threads t ON t.processlist_id = pl.id",
				"INNER JOIN performance_schema.events_statements_current es ON es.thread_id = t.thread_id",
				"WHERE pl.command NOT IN ('sleep', 'killed')",
				"AND pl.info NOT LIKE '%processlist%'",
				"AND pl.time >= 60",
				"AND pl.db IN ('test_db')",
				"ORDER BY pl.time DESC",
			},
		},
		{
			name: "different query limit duration",
			sniper: QuerySniper{
				QueryLimit: 5 * time.Minute,
				Schema:     "production",
			},
			wantContains: []string{
				"AND pl.time >= 300", // 5 minutes = 300 seconds
				"AND pl.db IN ('production')",
			},
		},
		{
			name: "fractional seconds get truncated to int",
			sniper: QuerySniper{
				QueryLimit: 1500 * time.Millisecond, // 1.5 seconds
				Schema:     "",
			},
			wantContains: []string{
				"AND pl.time >= 1", // 1.5 seconds truncated to 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := tt.sniper.generateHunterQuery()
			if err != nil {
				t.Errorf("generateHunterQuery() error = %v", err)

				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("generateHunterQuery() result missing expected string %q\nGot: %s", want, got)
				}
			}

			for _, unwanted := range tt.wantNotContain {
				if strings.Contains(got, unwanted) {
					t.Errorf("generateHunterQuery() result contains unwanted string %q\nGot: %s", unwanted, got)
				}
			}

			if strings.Contains(got, "\n") || strings.Contains(got, "\t") {
				t.Errorf("generateHunterQuery() result should not contain newlines or tabs\nGot: %s", got)
			}

			normalized := strings.Join(strings.Fields(got), " ")
			if got != normalized {
				t.Errorf("generateHunterQuery() result has excessive whitespace\nGot: %s\nWant: %s", got, normalized)
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
				DryRun: true,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
				}{
					"test_db": {
						Address:              "127.0.0.1:3306",
						Schema:               "production",
						Username:             "test_user",
						Password:             "test_pass",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
					},
				},
			},
			wantErr: false,
		},
		{
			name:   "successful sniper creation without schema",
			dbName: "analytics",
			settings: &configuration.Config{
				DryRun: false,
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
				}{
					"analytics": {
						Address:              "db.example.com:3306",
						Schema:               "", // Empty schema
						Username:             "analytics_user",
						Password:             "secret123",
						Interval:             45 * time.Second,
						LongQueryLimit:       300 * time.Second,
						LongTransactionLimit: 600 * time.Second,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := createSniper(tt.dbName, tt.settings)
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

			if got.DryRun != tt.settings.DryRun {
				t.Errorf("createSniper() DryRun = %v, want %v", got.DryRun, tt.settings.DryRun)
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

			if got.Connection != nil {
				got.Connection.Close()
			}
		})
	}
}

func TestCreateSniper_NonExistentDatabase(t *testing.T) {
	t.Parallel()

	settings := &configuration.Config{
		DryRun: false,
		Databases: map[string]struct {
			Address              string        `mapstructure:"address"`
			Schema               string        `mapstructure:"schema"`
			Username             string        `mapstructure:"username"`
			Password             string        `mapstructure:"password"`
			Interval             time.Duration `mapstructure:"interval"`
			LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
			LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
		}{
			"existing_db": {
				Address:              "127.0.0.1:3306",
				Schema:               "test",
				Username:             "user",
				Password:             "pass",
				Interval:             30 * time.Second,
				LongQueryLimit:       60 * time.Second,
				LongTransactionLimit: 120 * time.Second,
			},
		},
	}

	got, err := createSniper("non_existent_db", settings)
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

	if got.Connection != nil {
		got.Connection.Close()
	}
}
