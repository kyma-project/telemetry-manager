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
	configYAML, err := yaml.Marshal(config)
	require.NoError(t, err)

	goldenFilePath := filepath.Join("testdata", "config.yaml")
	if testutils.ShouldUpdateGoldenFiles() {
		testutils.UpdateGoldenFileYAML(t, goldenFilePath, configYAML)
		return
	}

	goldenFile, err := os.ReadFile(goldenFilePath)
	require.NoError(t, err, "failed to load golden file")
	require.Equal(t, string(goldenFile), string(configYAML))
}
