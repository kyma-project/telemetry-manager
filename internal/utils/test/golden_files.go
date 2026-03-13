package test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func ShouldUpdateGoldenFiles() bool {
	for _, arg := range os.Args {
		if arg == "-update-golden-files" || arg == "--update-golden-files" {
			return true
		}
	}

	return false
}

// UpdateGoldenFileYAML updates the golden file YAML config.
// Commit changes to golden files only if they are intentional and have been carefully reviewed.
func UpdateGoldenFileYAML(t *testing.T, filePath string, configYAML []byte) {
	err := os.WriteFile(filePath, configYAML, 0600)
	require.NoError(t, err, "failed to save YAML file")
}
