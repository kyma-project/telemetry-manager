package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMakeConfigMarshalling(t *testing.T) {
	config := MakeConfig()
	monitorConfigYaml, err := yaml.Marshal(config)
	require.NoError(t, err)

	goldenMonitoringConfigPath := filepath.Join("testdata", "config.yaml")
	goldenMonitoringFile, err := os.ReadFile(goldenMonitoringConfigPath)
	require.NoError(t, err, "failed to load golden monitoring file")
	require.Equal(t, string(goldenMonitoringFile), string(monitorConfigYaml))

}
