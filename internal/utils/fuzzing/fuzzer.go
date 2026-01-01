package fuzzing

import (
	"math/rand"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metafuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/intstr"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"
)

// getFuzzer returns a new fuzzer to be used for testing.
func getFuzzer(scheme *runtime.Scheme, funcs ...fuzzer.FuzzerFuncs) *randfill.Filler {
	funcs = append([]fuzzer.FuzzerFuncs{
		metafuzzer.Funcs,
		func(_ serializer.CodecFactory) []any {
			return []any{
				// Custom fuzzer for metav1.Time pointers which weren't
				// fuzzed and always resulted in `nil` values.
				// This implementation is somewhat similar to the one provided
				// in the metafuzzer.Funcs.
				func(input **metav1.Time, c randfill.Continue) {
					if c.Bool() {
						// Leave the Time sometimes nil to also get coverage for this case.
						return
					}

					if c.Bool() {
						// Set the Time sometimes empty to also get coverage for this case.
						*input = &metav1.Time{}
						return
					}

					var sec, nsec uint32
					c.Fill(&sec)
					c.Fill(&nsec)
					fuzzed := metav1.Unix(int64(sec), int64(nsec)).Rfc3339Copy()
					*input = &metav1.Time{Time: fuzzed.Time}
				},
				// Custom fuzzer for intstr.IntOrString which does not get fuzzed otherwise.
				func(in **intstr.IntOrString, c randfill.Continue) {
					if c.Bool() {
						// Leave the IntOrString sometimes nil to also get coverage for this case.
						return
					}

					if c.Bool() {
						// Set the IntOrString sometimes empty to also get coverage for this case.
						*in = &intstr.IntOrString{}
						return
					}

					*in = ptr.To(intstr.FromInt32(c.Int31n(50))) //nolint:mnd // fuzzing value
				},
			}
		},
	}, funcs...)

	return fuzzer.FuzzerFor(
		fuzzer.MergeFuzzerFuncs(funcs...),
		rand.NewSource(rand.Int63()), //nolint:gosec // only for fuzz test
		serializer.NewCodecFactory(scheme),
	)
}

// FuzzTestFuncInput contains input parameters
// for the FuzzTestFunc function.
type FuzzTestFuncInput struct {
	Scheme *runtime.Scheme

	Hub              conversion.Hub
	HubAfterMutation func(conversion.Hub)

	Spoke                      conversion.Convertible
	SpokeAfterMutation         func(convertible conversion.Convertible)
	SkipSpokeAnnotationCleanup bool

	FuzzerFuncs []fuzzer.FuzzerFuncs
}

// FuzzTestFunc returns a new testing function to be used in tests to make sure conversions between
// the Hub version of an object and an older version aren't lossy.
//
//nolint:gocognit // function was taken from https://github.com/kubernetes-sigs/cluster-api/blob/main/util/conversion/conversion.go
func FuzzTestFunc(input FuzzTestFuncInput) func(*testing.T) {
	if input.Scheme == nil {
		input.Scheme = clientgoscheme.Scheme
	}

	return func(t *testing.T) {
		t.Helper()
		// only testing spoke-hub-spoke since hub-spoke-hub is not guaranteed to be lossless (due to possible down-conversion data loss)
		t.Run("spoke-hub-spoke", func(t *testing.T) {
			g := gomega.NewWithT(t)
			fuzzer := getFuzzer(input.Scheme, input.FuzzerFuncs...)

			for range 10000 {
				// Create the spoke and fuzz it
				spokeBefore, ok := input.Spoke.DeepCopyObject().(conversion.Convertible)
				if !ok {
					t.Fatalf("spoke does not implement conversion.Convertible")
				}

				fuzzer.Fill(spokeBefore)

				// First convert spoke to hub
				hubCopy, ok := input.Hub.DeepCopyObject().(conversion.Hub)

				if !ok {
					t.Fatalf("spoke hub not implement conversion.Hub")
				}

				g.Expect(spokeBefore.ConvertTo(hubCopy)).To(gomega.Succeed())

				// Convert hub back to spoke and check if the resulting spoke is equal to the spoke before the round trip
				spokeAfter, ok := input.Spoke.DeepCopyObject().(conversion.Convertible)

				if !ok {
					t.Fatalf("spoke does not implement conversion.Convertible")
				}

				g.Expect(spokeAfter.ConvertFrom(hubCopy)).To(gomega.Succeed())

				if input.SpokeAfterMutation != nil {
					input.SpokeAfterMutation(spokeAfter)
				}

				if !apiequality.Semantic.DeepEqual(spokeBefore, spokeAfter) {
					diff := cmp.Diff(spokeBefore, spokeAfter)
					g.Expect(false).To(gomega.BeTrue(), diff)
				}
			}
		})
	}
}
