// Package servicenamebundle deploys a set of Kubernetes resources
// needed for testing service name enrichment.
package servicenamebundle

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	kitk8s "github.com/kyma-project/telemetry-manager/test/testkit/k8s"
)

type SignalType string

const (
	SignalTypeTraces  = "traces"
	SignalTypeMetrics = "metrics"
)

const (
	// KubeAppLabelValue is the value for the Kubernetes-specific app label
	KubeAppLabelValue = "kube-workload"

	// AppLabelValue is the value for the general app label
	AppLabelValue = "workload"

	// Predefined names for Kubernetes resources
	PodWithBothLabelsName = "pod-with-both-app-labels"
	PodWithAppLabelName   = "pod-with-app-label"
	DeploymentName        = "deployment"
	StatefulSetName       = "stateful-set"
	DaemonSetName         = "daemon-set"
	JobName               = "job"
)

// K8sObjects generates and returns a list of Kubernetes objects
// that are set up for testing service name enrichment.
func K8sObjects(namespace string, signalType SignalType) []client.Object {
	podSpec := makeTelemetryGenPodSpec(signalType)
	return []client.Object{
		kitk8s.NewPod(PodWithBothLabelsName, namespace).
			WithLabel("app.kubernetes.io/name", KubeAppLabelValue).
			WithLabel("app", AppLabelValue).
			WithPodSpec(podSpec).
			K8sObject(),
		kitk8s.NewPod(PodWithAppLabelName, namespace).
			WithLabel("app", AppLabelValue).
			WithPodSpec(podSpec).
			K8sObject(),
		kitk8s.NewDeployment(DeploymentName, namespace).WithPodSpec(podSpec).K8sObject(),
		kitk8s.NewStatefulSet(StatefulSetName, namespace).WithPodSpec(podSpec).K8sObject(),
		kitk8s.NewDaemonSet(DaemonSetName, namespace).WithPodSpec(podSpec).K8sObject(),
		kitk8s.NewJob(JobName, namespace).WithPodSpec(podSpec).K8sObject(),
	}
}
