package configuration

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/goforj/godump"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	ErrNoCredentialsFile       = errors.New("no credentials file specified")
	ErrNoDatabasesConfigured   = errors.New("no databases have been configured")
	ErrEmptyUsername           = errors.New("empty username")
	ErrEmptyPassword           = errors.New("empty password")
	ErrEmptyAddress            = errors.New("empty address")
	ErrInvalidAddressFormat    = errors.New("invalid address format")
	ErrEmptySchema             = errors.New("empty schema")
	ErrInvalidInterval         = errors.New("invalid interval")
	ErrInvalidQueryLimit       = errors.New("invalid query limit")
	ErrInvalidTransactionLimit = errors.New("invalid transaction limit")
)

// This struct is sorted by datatype to satisfy the fieldalignment linter rule.
type Config struct {
	Databases map[string]struct {
		Address              string        `mapstructure:"address"`
		Schema               string        `mapstructure:"schema"` // TODO(kuzmik): add support for multiple schemas.
		Username             string        `mapstructure:"username"`
		Password             string        `mapstructure:"password"`
		Interval             time.Duration `mapstructure:"interval"`
		LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
		LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
		DryRun               bool          `mapstructure:"dry_run"`
	} `mapstructure:"databases"`
	CredentialFile string `mapstructure:"credential_file"`
	Log            struct {
		Format        string `mapstructure:"format"`
		Level         string `mapstructure:"level"`
		IncludeCaller bool   `mapstructure:"include_caller"`
	} `mapstructure:"log"`
	SafeMode bool `mapstructure:"safe-mode"`
}

// Configure loads the configuration from the specified file, and merges the
// credentials file into the configuration.
func Configure() (*Config, error) {
	if file := os.Getenv("SNIPER_CONFIG_FILE"); file != "" {
		// if the config file path is specified in the env, load that
		viper.SetConfigFile(file)
	} else {
		// otherwise setup some default locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("configs")
	}

	// setup the pflags for the application.
	pflag.Bool("show-config", false, "Show the config; valid values are [true OR false], defaults to false")
	pflag.Bool("safe-mode", false, "Enable safe mode globally, and will override all database dry_run settings; valid values are [true OR false], defaults to false")
	pflag.Bool("log.include_caller", false, "Include the caller in the logs; valid values are [true OR false], defaults to false")
	pflag.String("log.format", "JSON", "Format of the logs; valid values are [JSON OR TEXT], defaults to JSON")
	pflag.String("log.level", "INFO", "the log level for the agent; valid values are [TRACE, DEBUG, INFO, WARN, ERROR, FATAL], defaults to INFO")

	// read the config file, or return an error if it doesn't exist.
	err := viper.ReadInConfig()
	if err != nil {
		errVal := viper.ConfigFileNotFoundError{}
		if ok := errors.As(err, &errVal); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// load the credentials config and merge it into the existing configuration.
	var credentialsFile string
	if file := os.Getenv("SNIPER_CREDS_FILE"); file != "" {
		credentialsFile = file
	} else {
		credentialsFile = viper.GetViper().GetString("credential_file")
	}

	// no credentials file specified, return an error because without authentication, we can't do anything.
	if credentialsFile == "" {
		return nil, ErrNoCredentialsFile
	}

	viper.SetConfigFile(credentialsFile)

	err = viper.MergeInConfig()
	if err != nil {
		return nil, fmt.Errorf("error merging credentials config: %w", err)
	}

	pflag.Parse()

	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return nil, fmt.Errorf("error binding pflags: %w", err)
	}

	settings := &Config{}

	err = viper.Unmarshal(settings)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	// if the show-config flag is set, dump the redacted config and exit.
	if viper.GetBool("show-config") {
		godump.Dump(settings.Redact())

		os.Exit(0)
	}

	if settings.Databases == nil {
		return settings, ErrNoDatabasesConfigured
	}

	err = settings.Validate()
	if err != nil {
		return settings, err
	}

	return settings, nil
}

// Redact returns a copy of *Config, but in its place returns a copy of the config with
// the password redacted, for use when dumping the config to the console.
func (settings *Config) Redact() Config {
	redacted := *settings

	redacted.Databases = make(map[string]struct {
		Address              string        `mapstructure:"address"`
		Schema               string        `mapstructure:"schema"`
		Username             string        `mapstructure:"username"`
		Password             string        `mapstructure:"password"`
		Interval             time.Duration `mapstructure:"interval"`
		LongQueryLimit       time.Duration `mapstructure:"long_query_limit"`
		LongTransactionLimit time.Duration `mapstructure:"long_transaction_limit"`
		DryRun               bool          `mapstructure:"dry_run"`
	})

	for name, db := range settings.Databases {
		dbCopy := db
		dbCopy.Password = "[REDACTED]"
		redacted.Databases[name] = dbCopy
	}

	return redacted
}

func (settings *Config) Validate() error {
	if settings.Databases == nil {
		return ErrNoDatabasesConfigured
	}

	for name, db := range settings.Databases {
		if db.Username == "" {
			return fmt.Errorf("username is missing for database %s: %w", name, ErrEmptyUsername)
		}

		if db.Password == "" {
			return fmt.Errorf("password is missing for database %s: %w", name, ErrEmptyPassword)
		}

		if db.Address == "" {
			return fmt.Errorf("address is missing for database %s: %w", name, ErrEmptyAddress)
		}

		if !strings.Contains(db.Address, ":") {
			return fmt.Errorf("address %s must contain a port for database %s: %w", db.Address, name, ErrInvalidAddressFormat)
		}

		if db.Schema == "" {
			return fmt.Errorf("schema is missing for database %s: %w", name, ErrEmptySchema)
		}

		if db.Interval <= 0 {
			return fmt.Errorf("interval %d is invalid for database %s: %w", db.Interval, name, ErrInvalidInterval)
		}

		if db.LongQueryLimit <= 0 {
			return fmt.Errorf("long_query_limit %d is invalid for database %s: %w", db.LongQueryLimit, name, ErrInvalidQueryLimit)
		}

		if db.LongTransactionLimit < 0 {
			return fmt.Errorf("long_transaction_limit %d is invalid for database %s: %w", db.LongTransactionLimit, name, ErrInvalidTransactionLimit)
		}
	}

	return nil
}
