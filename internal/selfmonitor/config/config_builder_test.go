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
	configYaml, err := yaml.Marshal(config)
	require.NoError(t, err)

	goldenFilePath := filepath.Join("testdata", "config.yaml")
	goldenFile, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "failed to load golden file")
	require.Equal(t, string(goldenFile), string(configYaml))
}
