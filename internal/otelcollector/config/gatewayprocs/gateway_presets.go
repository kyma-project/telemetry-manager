package gatewayprocs

import (
	"fmt"

	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
)

type EnrichmentOpts struct {
	Enabled   bool
	PodLabels []PodLabel
}

type PodLabel struct {
	Key       string
	KeyPrefix string
}

func buildPodLabelPresets(presets EnrichmentOpts) []config.ExtractLabel {
	podLabelPresets := make([]config.ExtractLabel, 0)

	if presets.Enabled {
		for _, label := range presets.PodLabels {
			labelConfig := config.ExtractLabel{
				From:    "pod",
				TagName: "k8s.pod.label.$0",
			}

			if label.KeyPrefix != "" {
				labelConfig.KeyRegex = fmt.Sprintf("(%s.*)", label.KeyPrefix)
			} else {
				labelConfig.KeyRegex = fmt.Sprintf("(^%s$)", label.Key)
			}

			podLabelPresets = append(podLabelPresets, labelConfig)
		}
	}

	return podLabelPresets
}
