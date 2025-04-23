package configuration

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/yassinebenaid/godump"
)

type Config struct {
	Databases map[string]struct {
		Address        string        `mapstructure:"address"`
		Schema         string        `mapstructure:"schema"`
		Username       string        `mapstructure:"username"`
		Password       string        `mapstructure:"password"`
		Interval       time.Duration `mapstructure:"interval"`
		LongQueryLimit time.Duration `mapstructure:"long_query_limit"`
	} `mapstructure:"databases"`
	LogLevel    string `mapstructure:"log_level"`
	Credentials string `mapstructure:"credentials"`
}

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

	// read the config file, if it exists. if not, keep on truckin'
	if err := viper.ReadInConfig(); err != nil {
		errVal := viper.ConfigFileNotFoundError{}
		if ok := errors.As(err, &errVal); !ok {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// load the credentials config and merge it into the existing configuration.
	if file := os.Getenv("SNIPER_CREDS_FILE"); file != "" {
		viper.SetConfigFile(file)

		err := viper.MergeInConfig()
		if err != nil {
			return nil, fmt.Errorf("error merging credentials config: %w", err)
		}
	} else {
		if creds := viper.GetViper().GetString("credentials"); creds != "" {
			viper.SetConfigFile(creds)

			err := viper.MergeInConfig()
			if err != nil {
				return nil, fmt.Errorf("error merging credentials config: %w", err)
			}
		}
	}

	pflag.Bool("show-config", false, "Dump the configuration for debugging")

	err := pflag.CommandLine.MarkHidden("show-config")
	if err != nil {
		return nil, fmt.Errorf("error marking hidden flag: %w", err)
	}

	pflag.Parse()

	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return nil, fmt.Errorf("error binding pflags: %w", err)
	}

	// we are only dumping the config if the secret flag show-config is specified, because the config
	// contains the proxysql admin password
	if viper.GetViper().GetBool("show-config") {
		_ = godump.Dump(viper.GetViper().AllSettings())

		os.Exit(0)
	}

	settings := &Config{} //nolint:exhaustruct

	err = viper.Unmarshal(settings)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling config: %w", err)
	}

	return settings, nil
}
