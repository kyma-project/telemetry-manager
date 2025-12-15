package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	testutils "github.com/kyma-project/telemetry-manager/internal/utils/test"
)

func TestMakeConfigMarshalling(t *testing.T) {
	config := MakeConfig(BuilderConfig{
		ScrapeNamespace:        "kyma-system",
		AlertmanagerWebhookURL: "webhook.svc.local:9090",
		ConfigPath:             "/dummy-configpath/",
		AlertRuleFileName:      "dymma-alerts.yml",
	})
	configYaml, err := yaml.Marshal(config)
	require.NoError(t, err)

	goldenFilePath := filepath.Join("testdata", "config.yaml")
	if testutils.ShouldUpdateGoldenFiles() {
		err = os.WriteFile(goldenFilePath, configYaml, 0600)
		require.NoError(t, err, "failed to overwrite golden file")

		t.Fatalf("Golden file %s has been saved, please verify it and set the overwriteGoldenFile flag to false", goldenFilePath)

		return
	}

	goldenFile, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "failed to load golden file")
	require.Equal(t, string(goldenFile), string(configYaml))
}
