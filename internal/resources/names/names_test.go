package names

import (
	"strings"
	"testing"
)

func TestPipelineSyncerNames(t *testing.T) {
	syncers := map[string]string{
		"LogPipelineSync":    LogPipelineSync,
		"MetricPipelineSync": MetricPipelineSync,
		"TracePipelineSync":  TracePipelineSync,
	}

	for constName, value := range syncers {
		if !strings.HasSuffix(value, "-syncer") {
			t.Errorf("%s = %q: must end in \"-syncer\", got %q", constName, value, value)
		}

		if strings.Contains(value, "-sync-") {
			t.Errorf("%s = %q: must not contain redundant \"-sync-\" infix", constName, value)
		}
	}
}
