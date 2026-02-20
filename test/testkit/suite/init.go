package suite

import (
	"flag"

	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	istiotelemetryclientv1 "istio.io/client-go/pkg/apis/telemetry/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
	operatorv1beta1 "github.com/kyma-project/telemetry-manager/apis/operator/v1beta1"
	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
)

var (
	scheme = runtime.NewScheme()
)

//nolint:gochecknoinits // Runtime's scheme addition is required.
func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv1beta1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1alpha1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1beta1.AddToScheme(scheme))
	utilruntime.Must(istiosecurityclientv1.AddToScheme(scheme))
	utilruntime.Must(istionetworkingclientv1.AddToScheme(scheme))
	utilruntime.Must(istiotelemetryclientv1.AddToScheme(scheme))

	// Register test flags
	flag.StringVar(&labelFilterFlag, "labels", "", "Label filter expression (e.g., 'log-agent and istio')")
	flag.BoolVar(&doNotExecuteFlag, "do-not-execute", false, "Dry-run mode: print test info without executing")
	flag.BoolVar(&printLabelsFlag, "print-labels", false, "Print test labels in structured format (pipe-separated)")
}
