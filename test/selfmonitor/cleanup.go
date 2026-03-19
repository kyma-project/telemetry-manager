package selfmonitor

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/utils/ptr"
)

const (
	fluentBitHostPathCleanupName = "telemetry-fluent-bit-hostpath-cleanup"
	fluentBitHostPath            = "/var/telemetry-fluent-bit"
	cleanupMountPath             = "/cleanup"
)

// TelemetryNamespace is the namespace where telemetry components run in the test cluster.
const TelemetryNamespace = "kyma-system"

// FluentBitHostPathCleanupDaemonSet returns a DaemonSet that holds the Fluent Bit hostPath mount during the test.
// Add it to the resources passed to CreateObjects in Fluent Bit tests. When CreateObjects' t.Cleanup deletes the
// DaemonSet, the pod's preStop hook runs rm -rf on the hostPath so the next test sees a clean buffer directory.
func FluentBitHostPathCleanupDaemonSet(namespace string) client.Object {
	return fluentBitHostPathCleanupDaemonSet(namespace)
}

func fluentBitHostPathCleanupDaemonSet(namespace string) *appsv1.DaemonSet {
	volumeName := "hostpath"
	grace := int64(60)
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluentBitHostPathCleanupName,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": fluentBitHostPathCleanupName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": fluentBitHostPathCleanupName},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &grace,
					Containers: []corev1.Container{
						{
							Name:  "sleep",
							Image: "busybox",
							Command: []string{"sh", "-c", "sleep 3600"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: volumeName, MountPath: cleanupMountPath},
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh", "-c", "rm -rf " + cleanupMountPath + "/*"},
									},
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: volumeName,
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: fluentBitHostPath,
									Type: ptr.To(corev1.HostPathDirectoryOrCreate),
								},
							},
						},
					},
					Tolerations: []corev1.Toleration{
						{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute},
						{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule},
					},
				},
			},
		},
	}
}
