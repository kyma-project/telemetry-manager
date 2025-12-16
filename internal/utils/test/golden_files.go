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

func UpdateGoldenFile(t *testing.T, filePath string, configYAML []byte) {
	err := os.WriteFile(filePath, configYAML, 0600)
	require.NoError(t, err, "failed to update golden file")
}
