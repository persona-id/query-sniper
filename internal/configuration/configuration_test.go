package configuration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestConfigFile(t *testing.T) {
	tests := []struct {
		name     string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name:    "valid configuration",
			wantErr: false,
			validate: func(t *testing.T, config *Config) {
				if got := config.Credentials; got != "testdata/test_credentials.yaml" {
					t.Errorf("Config.Credentials = %v, want %v", got, "testdata/test_credentials.yaml")
				}

				primary := config.Databases["test_primary"]
				if got := primary.Address; got != "127.0.0.1:3306" {
					t.Errorf("Primary DB Address = %v, want %v", got, "127.0.0.1:3306")
				}
				if got := primary.Username; got != "primary_user" {
					t.Errorf("Primary DB Username = %v, want %v", got, "primary_user")
				}
				if got := primary.ReplicaLagLimit; got != 15*time.Minute {
					t.Errorf("Primary DB ReplicaLagLimit = %v, want %v", got, 15*time.Minute)
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
				if got := replica.HLLLimit; got != 120 {
					t.Errorf("Replica DB HLLLimit = %v, want %v", got, 120)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()

			currentDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}

			configFilePath := filepath.Join(currentDir, "../../testdata/test_config.yaml")
			credentialsFilePath := filepath.Join(currentDir, "../../testdata/test_credentials.yaml")

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
