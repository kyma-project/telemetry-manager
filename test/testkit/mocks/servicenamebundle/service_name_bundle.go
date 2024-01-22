// Package servicenamebundle deploys a set of Kubernetes resources
// needed for testing service name enrichment.
package servicenamebundle

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
	"github.com/kyma-project/telemetry-manager/test/testkit/mocks/telemetrygen"
)

const (
	// KubeAppLabelValue is the value for the Kubernetes-specific app label
	KubeAppLabelValue = "kube-workload"

	// AppLabelValue is the value for the general app label
	AppLabelValue = "workload"

	// Predefined names for Kubernetes resources
	PodWithBothLabelsName                             = "pod-with-both-app-labels" //#nosec G101 -- This is a false positive
	PodWithAppLabelName                               = "pod-with-app-label"
	DeploymentName                                    = "deployment"
	StatefulSetName                                   = "stateful-set"
	DaemonSetName                                     = "daemon-set"
	JobName                                           = "job"
	PodWithNoLabelsName                               = "pod-with-no-labels"
	PodWithUnknownServiceName                         = "pod-with-unknown-service"
	PodWithUnknownServicePatternName                  = "pod-with-unknown-service-pattern"
	PodWithInvalidStartForUnknownServicePatternName   = "pod-with-invalid-start-for-unknown-service-pattern"
	PodWithInvalidEndForUnknownServicePatternName     = "pod-with-invalid-end-for-unknown-service-pattern"
	PodWithMissingProcessForUnknownServicePatternName = "pod-with-missing-process-for-unknown-service-pattern"

	// Invalid values for the unknown_service:<process.executable.name> pattern
	AttrWithInvalidStartForUnknownServicePattern   = "test_unknown_service"
	AttrWithInvalidEndForUnknownServicePattern     = "unknown_service_test"
	AttrWithMissingProcessForUnknownServicePattern = "unknown_service:"
)

// K8sObjects generates and returns a list of Kubernetes objects
// that are set up for testing service name enrichment.
func K8sObjects(namespace string, signalType telemetrygen.SignalType) []client.Object {
	podSpecWithUndefinedService := telemetrygen.PodSpec(signalType, "")
	podSpecWithUnknownService := telemetrygen.PodSpec(signalType, "unknown_service")
	podSpecWithUnknownServicePattern := telemetrygen.PodSpec(signalType, "unknown_service:bash")
	podSpecWithInvalidStartForUnknownServicePattern := telemetrygen.PodSpec(signalType, AttrWithInvalidStartForUnknownServicePattern)
	podSpecWithInvalidEndForUnknownServicePattern := telemetrygen.PodSpec(signalType, AttrWithInvalidEndForUnknownServicePattern)
	podSpecWithMissingProcessForUnknownServicePattern := telemetrygen.PodSpec(signalType, AttrWithMissingProcessForUnknownServicePattern)
	return []client.Object{
		kitk8s.NewPod(PodWithBothLabelsName, namespace).
			WithLabel("app.kubernetes.io/name", KubeAppLabelValue).
			WithLabel("app", AppLabelValue).
			WithPodSpec(podSpecWithUndefinedService).
			K8sObject(),
		kitk8s.NewPod(PodWithAppLabelName, namespace).
			WithLabel("app", AppLabelValue).
			WithPodSpec(podSpecWithUndefinedService).
			K8sObject(),
		kitk8s.NewDeployment(DeploymentName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8s.NewStatefulSet(StatefulSetName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8s.NewDaemonSet(DaemonSetName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8s.NewJob(JobName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8s.NewPod(PodWithNoLabelsName, namespace).WithPodSpec(podSpecWithUndefinedService).K8sObject(),
		kitk8s.NewPod(PodWithUnknownServicePatternName, namespace).WithPodSpec(podSpecWithUnknownServicePattern).K8sObject(),
		kitk8s.NewPod(PodWithUnknownServiceName, namespace).WithPodSpec(podSpecWithUnknownService).K8sObject(),
		kitk8s.NewPod(PodWithInvalidStartForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithInvalidStartForUnknownServicePattern).K8sObject(),
		kitk8s.NewPod(PodWithInvalidEndForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithInvalidEndForUnknownServicePattern).K8sObject(),
		kitk8s.NewPod(PodWithMissingProcessForUnknownServicePatternName, namespace).WithPodSpec(podSpecWithMissingProcessForUnknownServicePattern).K8sObject(),
	}
}
