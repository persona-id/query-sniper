package configuration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestConfigFile(t *testing.T) {
	viper.Reset()

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Get the absolute path to the test config files, to prevent copying them around in the test.
	configFilePath := filepath.Join(currentDir, "../../test/test_config.yaml")
	credentialsFilePath := filepath.Join(currentDir, "../../test/test_credentials.yaml")

	t.Setenv("SNIPER_CONFIG_FILE", configFilePath)
	t.Setenv("SNIPER_CREDS_FILE", credentialsFilePath)

	config, err := Configure()
	assert.NoError(t, err, "Configuration should not return an error")

	assert.Equal(t, config.Credentials, "test/test_credentials.yaml")

	assert.Equal(t, "127.0.0.1:3306", config.Databases["test_primary"].Address)
	assert.Equal(t, "primary_user", config.Databases["test_primary"].Username)
	assert.Equal(t, 15*time.Minute, config.Databases["test_primary"].ReplicaLagLimit)
	assert.Equal(t, 60*time.Second, config.Databases["test_primary"].LongQueryLimit)

	assert.Equal(t, "test_schema", config.Databases["test_primary"].Schema)
	assert.Equal(t, "replica_pass", config.Databases["test_replica"].Password)
	assert.Equal(t, 120, config.Databases["test_replica"].HLLLimit)
}
