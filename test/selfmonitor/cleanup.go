package selfmonitor

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"k8s.io/utils/ptr"

	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const (
	fluentBitHostPathCleanupName = "telemetry-fluent-bit-hostpath-cleanup"
	fluentBitHostPath            = "/var/telemetry-fluent-bit"
	cleanupMountPath             = "/cleanup"
	cleanupWaitTimeout           = 2 * time.Minute
	cleanupPollInterval          = 5 * time.Second
)

// TelemetryNamespace is the namespace where telemetry components run in the test cluster.
const TelemetryNamespace = "kyma-system"

// WaitForFluentBitDaemonSetGone blocks until the Fluent Bit DaemonSet is deleted in the given namespace.
// Call this before CreateObjects in Fluent Bit tests so the hostPath cleanup DaemonSet can run on a clean slate.
func WaitForFluentBitDaemonSetGone(ctx context.Context, k8sClient client.Client, namespace string) error {
	deadline := time.Now().Add(cleanupWaitTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		ds := &appsv1.DaemonSet{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: names.FluentBit, Namespace: namespace}, ds)
		if err != nil {
			if client.IgnoreNotFound(err) == nil {
				return nil
			}
			return err
		}
		time.Sleep(cleanupPollInterval)
	}
	return fmt.Errorf("timeout waiting for DaemonSet %s/%s to be deleted", namespace, names.FluentBit)
}

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
