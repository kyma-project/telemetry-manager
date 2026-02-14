package suite

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kyma-project/telemetry-manager/test/testkit/kubeprep"
)

func TestInferRequirementsFromLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected kubeprep.Config
	}{
		{
			name:   "no labels - defaults",
			labels: []string{},
			expected: kubeprep.Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "istio label",
			labels: []string{LabelIstio},
			expected: kubeprep.Config{
				InstallIstio:            true,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "experimental label",
			labels: []string{LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      true,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "istio and experimental",
			labels: []string{LabelIstio, LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:            true,
				OperateInFIPSMode:       false,
				EnableExperimental:      true,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "unknown labels ignored",
			labels: []string{"unknown-label", "another-unknown"},
			expected: kubeprep.Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "mixed known and unknown labels",
			labels: []string{"unknown", LabelIstio, "another-unknown", LabelExperimental},
			expected: kubeprep.Config{
				InstallIstio:            true,
				OperateInFIPSMode:       false,
				EnableExperimental:      true,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
		{
			name:   "other labels don't affect config",
			labels: []string{LabelLogAgent, LabelFluentBit},
			expected: kubeprep.Config{
				InstallIstio:            false,
				OperateInFIPSMode:       false,
				EnableExperimental:      false,
				CustomLabelsAnnotations: false,
				SkipManagerDeployment:   false,
				SkipPrerequisites:       false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := InferRequirementsFromLabels(tt.labels)
			require.Equal(t, tt.expected, actual)
		})
	}
}
