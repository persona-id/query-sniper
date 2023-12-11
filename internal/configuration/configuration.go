package configuration

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	LogLevel    string `mapstructure:"log_level"`
	Credentials string `mapstructure:"credentials"`
	Databases   map[string]struct {
		Address         string        `mapstructure:"address"`
		Schema          string        `mapstructure:"schema"`
		Username        string        `mapstructure:"username"`
		Password        string        `mapstructure:"password"`
		ReplicaLagLimit time.Duration `mapstructure:"replica_lag_limit"`
		HLLLimit        int           `mapstructure:"hll_limit"`
		Interval        time.Duration `mapstructure:"interval"`
		LongQueryLimit  time.Duration `mapstructure:"long_query_limit"`
	} `mapstructure:"databases"`
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
			return nil, err
		}
	}

	// load the credentials config and merge it into the existing configuration.
	if file := os.Getenv("SNIPER_CREDS_FILE"); file != "" {
		viper.SetConfigFile(file)

		err := viper.MergeInConfig()
		if err != nil {
			return nil, err
		}
	} else {
		if creds := viper.GetViper().GetString("credentials"); creds != "" {
			viper.SetConfigFile(creds)

			err := viper.MergeInConfig()
			if err != nil {
				return nil, err
			}
		}
	}

	pflag.Bool("show-config", false, "Dump the configuration for debugging")

	err := pflag.CommandLine.MarkHidden("show-config")
	if err != nil {
		return nil, err
	}

	pflag.Parse()

	err = viper.BindPFlags(pflag.CommandLine)
	if err != nil {
		return nil, err
	}

	// we are only dumping the config if the secret flag show-config is specified, because the config
	// contains the proxysql admin password
	if viper.GetViper().GetBool("show-config") {
		fmt.Println("settings", viper.GetViper().AllSettings())
	}

	settings := &Config{}

	err = viper.Unmarshal(settings)
	if err != nil {
		return nil, err
	}

	return settings, nil
}
