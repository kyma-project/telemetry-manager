package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/utils/fuzzing"
)

func TestLogPipelineConversion(t *testing.T) {
	t.Run("spoke-hub-spoke", fuzzing.FuzzTestFunc(fuzzing.FuzzTestFuncInput{
		Hub:   &telemetryv1beta1.LogPipeline{},
		Spoke: &LogPipeline{},
		SpokeAfterMutation: func(spoke conversion.Convertible) {
			lp := spoke.(*LogPipeline)
			if lp.Spec.Input.Application != nil {
				lp.Spec.Input.Application.Namespaces.Include = sanitizeNamespaceNames(lp.Spec.Input.Application.Namespaces.Include)
				lp.Spec.Input.Application.Namespaces.Exclude = sanitizeNamespaceNames(lp.Spec.Input.Application.Namespaces.Exclude)
			}
		},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{
			// Custom fuzzer for NamespaceSelector to generate only valid namespace names
			func(_ serializer.CodecFactory) []any {
				return []any{
					func(nsSelector *NamespaceSelector, c randfill.Continue) {
						if c.Bool() {
							// Leave the NamespaceSelector sometimes nil to also get coverage for this case.
							return
						}

						if c.Bool() {
							// Set the NamespaceSelector sometimes empty to also get coverage for this case.
							*nsSelector = NamespaceSelector{}
							return
						}

						*nsSelector = NamespaceSelector{
							Include: generateValidNamespaceNames(c),
							Exclude: generateValidNamespaceNames(c),
						}
					},
				}
			},
			// Custom fuzzer for LogPipelineNamespaceSelector to generate only valid namespace names
			func(_ serializer.CodecFactory) []any {
				return []any{
					func(nsSelector *LogPipelineNamespaceSelector, c randfill.Continue) {
						if c.Bool() {
							// Leave the LogPipelineNamespaceSelector sometimes nil to also get coverage for this case.
							return
						}

						if c.Bool() {
							// Set the LogPipelineNamespaceSelector sometimes empty to also get coverage for this case.
							*nsSelector = LogPipelineNamespaceSelector{}
							return
						}

						*nsSelector = LogPipelineNamespaceSelector{
							Include: generateValidNamespaceNames(c),
							Exclude: generateValidNamespaceNames(c),
						}
					},
				}
			},
			func(_ serializer.CodecFactory) []any {
				return []any{
					func(nsSelector *telemetryv1beta1.NamespaceSelector, c randfill.Continue) {
						if c.Bool() {
							// Leave the NamespaceSelector sometimes nil to also get coverage for this case.
							return
						}

						if c.Bool() {
							// Set the NamespaceSelector sometimes empty to also get coverage for this case.
							*nsSelector = telemetryv1beta1.NamespaceSelector{}
							return
						}

						*nsSelector = telemetryv1beta1.NamespaceSelector{
							Include: generateValidNamespaceNames(c),
							Exclude: generateValidNamespaceNames(c),
						}
					},
				}
			},
		},
	}))
}

// generateValidNamespaceNames creates a slice of valid Kubernetes namespace names for fuzzing.
func generateValidNamespaceNames(c randfill.Continue) []string {
	count := c.Intn(5)
	if count == 0 {
		// Sometimes return nil or empty slice
		if c.Bool() {
			return nil
		}

		return []string{}
	}

	names := make([]string, 0, count)
	for range count {
		// Generate only valid namespace names
		nameLen := c.Intn(63) + 1

		nameRunes := make([]rune, nameLen)
		for j := range nameLen {
			var r rune

			switch pos := c.Intn(36); {
			case pos < 26:
				r = rune('a' + pos)
			case pos < 36:
				r = rune('0' + (pos - 26))
			}
			// Hyphens are allowed but not at the beginning or end
			if j != 0 && j != nameLen-1 && c.Intn(10) < 2 {
				r = '-'
			}

			nameRunes[j] = r
		}

		names = append(names, string(nameRunes))
	}

	return names
}
