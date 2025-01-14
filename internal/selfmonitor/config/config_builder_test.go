package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestMakeConfigMarshalling(t *testing.T) {
	config := MakeConfig(BuilderConfig{
		ScrapeNamespace:   "kyma-system",
		WebhookURL:        "http://webhook:9090",
		ConfigPath:        "/dummy-configpath/",
		AlertRuleFileName: "dymma-alerts.yml",
	})
	monitorConfigYaml, err := yaml.Marshal(config)
	require.NoError(t, err)

	goldenMonitoringConfigPath := filepath.Join("testdata", "config.yaml")
	goldenMonitoringFile, err := os.ReadFile(goldenMonitoringConfigPath)
	require.NoError(t, err, "failed to load golden monitoring file")
	require.Equal(t, string(goldenMonitoringFile), string(monitorConfigYaml))
}
