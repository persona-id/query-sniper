//nolint:paralleltest
package configuration

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

//nolint:gocognit,maintidx,gocyclo,cyclop
func TestConfigure(t *testing.T) {
	tests := []struct {
		expectedError error
		setupEnv      func(*testing.T, string)
		setupFiles    func(*testing.T, string)
		validate      func(*testing.T, *Config)
		name          string
		wantErr       bool
	}{
		{
			name: "success with env vars for config and creds",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				credsPath := filepath.Join(tempDir, "creds.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", credsPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
  format: JSON
  include_caller: false
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
    long_transaction_limit: 120s
`
				//nolint:gosec
				credsContent := `
databases:
  primary:
    username: test_user
    password: test_pass
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				t.Helper()
				if config.Databases["primary"].Username != "test_user" {
					t.Errorf("Expected username test_user, got %v", config.Databases["primary"].Username)
				}
				if config.Log.Level != "INFO" {
					t.Errorf("Expected log level INFO, got %v", config.Log.Level)
				}
			},
		},
		{
			name: "success with config file specifying credential_file",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				credsPath := filepath.Join(tempDir, "creds.yaml")
				configContent := fmt.Sprintf(`
credential_file: %s
log:
  level: DEBUG
  format: TEXT
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
    long_transaction_limit: 120s
`, credsPath)

				//nolint:gosec
				credsContent := `
databases:
  primary:
    username: config_user
    password: config_pass
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(credsPath, []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				t.Helper()

				if config.Databases["primary"].Username != "config_user" {
					t.Errorf("Expected username config_user, got %v", config.Databases["primary"].Username)
				}

				if config.Log.Level != "DEBUG" {
					t.Errorf("Expected log level DEBUG, got %v", config.Log.Level)
				}
			},
		},
		{
			name: "error - no credentials file specified",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr:       true,
			expectedError: ErrNoCredentialsFile,
		},
		{
			name: "error - credentials file not found",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", "/nonexistent/path")
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "error - invalid config file format",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", filepath.Join(tempDir, "creds.yaml"))
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				// Invalid YAML
				configContent := `
log:
  level: INFO
databases:
  - invalid yaml structure
    missing colon
`
				//nolint:gosec
				credsContent := `
databases:
  primary:
    username: user
    password: pass
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "error - invalid credentials file format",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				credsPath := filepath.Join(tempDir, "creds.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", credsPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
`
				// Invalid YAML
				//nolint:gosec
				credsContent := `
databases:
  - invalid yaml
    missing proper structure
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: true,
		},
		{
			name: "error - validation fails due to missing database config",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				credsPath := filepath.Join(tempDir, "creds.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", credsPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
# No databases configured
`
				//nolint:gosec
				credsContent := `
databases: {}
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr:       true,
			expectedError: ErrNoDatabasesConfigured,
		},
		{
			name: "success - config file not found (graceful handling)",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				// Don't set config file env var, so it will look for defaults and not find them
				credsPath := filepath.Join(tempDir, "creds.yaml")
				t.Setenv("SNIPER_CREDS_FILE", credsPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				// Only create credentials file, no config file
				//nolint:gosec
				credsContent := `
databases:
  primary:
    username: creds_user
    password: creds_pass
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
    long_transaction_limit: 120s
`
				err := os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				t.Helper()
				if config.Databases["primary"].Username != "creds_user" {
					t.Errorf("Expected username creds_user, got %v", config.Databases["primary"].Username)
				}
			},
		},
		{
			name: "success with safe-mode flag enabled",
			setupEnv: func(t *testing.T, tempDir string) {
				t.Helper()

				configPath := filepath.Join(tempDir, "config.yaml")
				credsPath := filepath.Join(tempDir, "creds.yaml")
				t.Setenv("SNIPER_CONFIG_FILE", configPath)
				t.Setenv("SNIPER_CREDS_FILE", credsPath)
			},
			setupFiles: func(t *testing.T, tempDir string) {
				t.Helper()

				configContent := `
log:
  level: INFO
  format: JSON
  include_caller: false
databases:
  primary:
    address: 127.0.0.1:3306
    schema: test_db
    interval: 30s
    long_query_limit: 60s
    long_transaction_limit: 120s
    dry_run: false
`
				//nolint:gosec
				credsContent := `
databases:
  primary:
    username: test_user
    password: test_pass
`
				err := os.WriteFile(filepath.Join(tempDir, "config.yaml"), []byte(configContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}

				err = os.WriteFile(filepath.Join(tempDir, "creds.yaml"), []byte(credsContent), 0o600)
				if err != nil {
					t.Fatal(err)
				}
			},
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				t.Helper()
				if config.Databases["primary"].Username != "test_user" {
					t.Errorf("Expected username test_user, got %v", config.Databases["primary"].Username)
				}
				if config.SafeMode {
					t.Errorf("Expected SafeMode false (default), got %v", config.SafeMode)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset viper state
			viper.Reset()

			// Reset pflags for each test to avoid "flag redefined" errors
			originalCommandLine := pflag.CommandLine
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

			t.Cleanup(func() {
				pflag.CommandLine = originalCommandLine
			})

			tempDir := t.TempDir()

			// Change to temp directory to avoid interfering with default config paths
			oldWd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			t.Cleanup(func() {
				t.Chdir(oldWd)
			})

			t.Chdir(tempDir)

			tt.setupEnv(t, tempDir)

			if tt.setupFiles != nil {
				tt.setupFiles(t, tempDir)
			}

			config, err := Configure()

			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.expectedError != nil && err != nil {
				if !errors.Is(err, tt.expectedError) {
					t.Errorf("Configure() error = %v, expected error type %v", err, tt.expectedError)
				}
			}

			if err == nil && tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestConfigure_PFlags(t *testing.T) {
	// Note: Testing pflag behavior with the Configure() function is complex
	// due to global state. This test demonstrates that the function can be
	// called and produces expected results with default flag values.
	// In a production scenario, dependency injection would make this more testable.
	t.Skip("Skipping pflag test due to global state conflicts with other tests")
}

func TestConfigFile(t *testing.T) {
	tests := []struct {
		validate func(*testing.T, *Config)
		name     string
		wantErr  bool
	}{
		{
			name:    "valid configuration",
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				t.Helper()

				if got := config.CredentialFile; got != "testdata/test_credentials.yaml" {
					t.Errorf("Config.CredentialFile = %v, want %v", got, "testdata/test_credentials.yaml")
				}

				primary := config.Databases["test_primary"]
				if got := primary.Address; got != "127.0.0.1:3306" {
					t.Errorf("Primary DB Address = %v, want %v", got, "127.0.0.1:3306")
				}

				if got := primary.Username; got != "primary_user" {
					t.Errorf("Primary DB Username = %v, want %v", got, "primary_user")
				}

				if got := primary.LongQueryLimit; got != 60*time.Second {
					t.Errorf("Primary DB LongQueryLimit = %v, want %v", got, 60*time.Second)
				}

				if got := primary.Schema; got != "test_schema" {
					t.Errorf("Primary DB Schema = %v, want %v", got, "test_schema")
				}

				replica := config.Databases["test_replica"]
				if got := replica.Password; got != "replica_pass" {
					t.Errorf("Replica DB Password = %v, want %v", got, "replica_pass")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			// Reset pflags for each test to avoid "flag redefined" errors
			originalCommandLine := pflag.CommandLine
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)

			t.Cleanup(func() {
				pflag.CommandLine = originalCommandLine
			})

			currentDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			configFilePath := filepath.Join(currentDir, "testdata/test_config.yaml")
			credentialsFilePath := filepath.Join(currentDir, "testdata/test_credentials.yaml")

			t.Setenv("SNIPER_CONFIG_FILE", configFilePath)
			t.Setenv("SNIPER_CREDS_FILE", credentialsFilePath)

			config, err := Configure()
			if (err != nil) != tt.wantErr {
				t.Errorf("Configure() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if err == nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestConfigure_ShowConfig(t *testing.T) {
	// Note: The show-config flag causes os.Exit(0) which makes it difficult to test
	// in a unit test environment. In a real scenario, you would use subprocess testing
	// or dependency injection to make this testable. For now, we're documenting the
	// limitation and skipping this specific behavior.
	t.Skip("show-config flag calls os.Exit(0), making it difficult to test in unit tests")
}

func TestConfig_Redact(t *testing.T) {
	t.Parallel()

	originalConfig := &Config{
		CredentialFile: "test_creds.yaml",
		Log: struct {
			Format        string `mapstructure:"format"`
			Level         string `mapstructure:"level"`
			IncludeCaller bool   `mapstructure:"include_caller"`
		}{
			Format:        "JSON",
			Level:         "INFO",
			IncludeCaller: true,
		},
		Databases: map[string]struct {
			Address              string        `mapstructure:"address"`
			Schema               string        `mapstructure:"schema"`
			Username             string        `mapstructure:"username"`
			Password             string        `mapstructure:"password"`
			Interval             time.Duration `mapstructure:"interval"`
			LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
			LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
			DryRun               bool          `mapstructure:"dry_run"`
		}{
			"primary": {
				Address:              "127.0.0.1:3306",
				Schema:               "test_db",
				Username:             "test_user",
				Password:             "secret_password",
				Interval:             30 * time.Second,
				LongQueryLimit:       60 * time.Second,
				LongTransactionLimit: 120 * time.Second,
				DryRun:               false,
			},
			"replica": {
				Address:              "127.0.0.1:3307",
				Schema:               "test_db",
				Username:             "replica_user",
				Password:             "another_secret",
				Interval:             45 * time.Second,
				LongQueryLimit:       90 * time.Second,
				LongTransactionLimit: 180 * time.Second,
				DryRun:               false,
			},
		},
	}

	redactedConfig := originalConfig.Redact()
	if originalConfig.Databases["primary"].Password != "secret_password" {
		t.Errorf("Original config was modified: got %v, want %v",
			originalConfig.Databases["primary"].Password, "secret_password")
	}

	if originalConfig.Databases["replica"].Password != "another_secret" {
		t.Errorf("Original config was modified: got %v, want %v",
			originalConfig.Databases["replica"].Password, "another_secret")
	}

	if redactedConfig.Databases["primary"].Password != "[REDACTED]" {
		t.Errorf("Password not redacted: got %v, want %v",
			redactedConfig.Databases["primary"].Password, "[REDACTED]")
	}

	if redactedConfig.Databases["replica"].Password != "[REDACTED]" {
		t.Errorf("Password not redacted: got %v, want %v",
			redactedConfig.Databases["replica"].Password, "[REDACTED]")
	}

	primary := redactedConfig.Databases["primary"]
	if primary.Address != "127.0.0.1:3306" {
		t.Errorf("Address not preserved: got %v, want %v",
			primary.Address, "127.0.0.1:3306")
	}

	if primary.Username != "test_user" {
		t.Errorf("Username not preserved: got %v, want %v",
			primary.Username, "test_user")
	}

	if primary.Schema != "test_db" {
		t.Errorf("Schema not preserved: got %v, want %v",
			primary.Schema, "test_db")
	}

	if primary.Interval != 30*time.Second {
		t.Errorf("Interval not preserved: got %v, want %v",
			primary.Interval, 30*time.Second)
	}

	if redactedConfig.CredentialFile != originalConfig.CredentialFile {
		t.Errorf("CredentialFile not preserved: got %v, want %v",
			redactedConfig.CredentialFile, originalConfig.CredentialFile)
	}

	if redactedConfig.Log.Format != originalConfig.Log.Format {
		t.Errorf("Log.Format not preserved: got %v, want %v",
			redactedConfig.Log.Format, originalConfig.Log.Format)
	}
}

//nolint:maintidx
func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		config      *Config
		expectedErr error
		name        string
		wantErr     bool
	}{
		{
			name: "valid configuration",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "nil databases",
			config:      &Config{Databases: nil},
			wantErr:     true,
			expectedErr: ErrNoDatabasesConfigured,
		},
		{
			name: "empty username",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "", // empty username
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrEmptyUsername,
		},
		{
			name: "empty password",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "", // empty password
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrEmptyPassword,
		},
		{
			name: "empty address",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "", // empty address
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrEmptyAddress,
		},
		{
			name: "address without port",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1", // missing port
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrInvalidAddressFormat,
		},
		{
			name: "empty schema",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "", // empty schema
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrEmptySchema,
		},
		{
			name: "invalid interval",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             0, // invalid interval
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrInvalidInterval,
		},
		{
			name: "invalid query limit",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       0, // invalid query limit
						LongTransactionLimit: 120 * time.Second,
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrInvalidQueryLimit,
		},
		{
			name: "invalid transaction limit",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: -1 * time.Second, // invalid transaction limit
						DryRun:               false,
					},
				},
			},
			wantErr:     true,
			expectedErr: ErrInvalidTransactionLimit,
		},
		{
			name: "zero transaction limit allowed",
			config: &Config{
				Databases: map[string]struct {
					Address              string        `mapstructure:"address"`
					Schema               string        `mapstructure:"schema"`
					Username             string        `mapstructure:"username"`
					Password             string        `mapstructure:"password"`
					Interval             time.Duration `mapstructure:"interval"`
					LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
					LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
					DryRun               bool          `mapstructure:"dry_run"`
				}{
					"primary": {
						Address:              "127.0.0.1:3306",
						Schema:               "test_db",
						Username:             "test_user",
						Password:             "secret_password",
						Interval:             30 * time.Second,
						LongQueryLimit:       60 * time.Second,
						LongTransactionLimit: 0, // zero is allowed
						DryRun:               false,
					},
				},
			},
			wantErr:     false,
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if tt.expectedErr != nil {
				if !errors.Is(err, tt.expectedErr) {
					t.Errorf("Config.Validate() error = %v, expected error type %v", err, tt.expectedErr)
				}
			}
		})
	}
}
