package kubeprep

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/test/testkit/periodic"
)

const (
	fluentBitHostPathCleanupDSName = "telemetry-fluent-bit-hostpath-cleanup"
	fluentBitHostPathOnNode        = "/var/telemetry-fluent-bit"
	fluentBitHostPathMountInPod    = "/cleanup"
	// Pin image tag for reproducible test cleanup pods (avoid floating :latest).
	fluentBitHostPathCleanupImage = "busybox:1.36"
)

// runtimeResourceSelector selects resources created at runtime by the telemetry-manager.
// Helm chart resources use "app.kubernetes.io/managed-by: kyma", while the manager
// stamps its runtime-created resources with "app.kubernetes.io/managed-by: telemetry-manager".
var runtimeResourceSelector = client.MatchingLabels{
	commonresources.LabelKeyK8sManagedBy: commonresources.LabelValueK8sManagedBy,
}

// WaitForManagedResourceCleanup waits until all runtime-created resources
// (labeled with app.kubernetes.io/managed-by=telemetry-manager) are deleted
// from the kyma-system namespace.
//
// This is intended to be called from t.Cleanup after pipeline resources have been
// deleted, giving the manager time to reconcile and remove dependent resources
// like collectors, agents, and the selfmonitor.
func WaitForManagedResourceCleanup(ctx context.Context, k8sClient client.Client) {
	Eventually(allRuntimeResourcesDeleted(ctx, k8sClient), periodic.EventuallyTimeout, periodic.DefaultInterval).Should(Succeed())
}

// CleanupFluentBitHostPath deploys a short-lived DaemonSet that mounts the Fluent Bit hostPath,
// then deletes it so the pod preStop hook clears /var/telemetry-fluent-bit on each node.
// Call after WaitForManagedResourceCleanup so the manager has removed the Fluent Bit DaemonSet.
//
// Errors from this function are intended to be non-fatal for callers (e.g. suite cleanup logs and continues).
// It therefore uses polling that returns errors on timeout instead of Gomega assertions, which would fail the test.
func CleanupFluentBitHostPath(ctx context.Context, k8sClient client.Client) error {
	key := types.NamespacedName{Name: fluentBitHostPathCleanupDSName, Namespace: kymaSystemNamespace}

	existing := &appsv1.DaemonSet{}
	if err := k8sClient.Get(ctx, key, existing); err == nil {
		if delErr := k8sClient.Delete(ctx, existing); client.IgnoreNotFound(delErr) != nil {
			return fmt.Errorf("delete stale hostpath cleanup daemonset: %w", delErr)
		}

		if err := waitUntil(ctx, periodic.EventuallyTimeout, periodic.DefaultInterval, func() (bool, error) {
			err := k8sClient.Get(ctx, key, &appsv1.DaemonSet{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			if err != nil {
				return false, fmt.Errorf("wait for stale hostpath cleanup daemonset gone: %w", err)
			}

			return false, nil
		}); err != nil {
			return err
		}
	} else if client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("get hostpath cleanup daemonset: %w", err)
	}

	ds := fluentBitHostPathCleanupDaemonSet(kymaSystemNamespace)
	if err := k8sClient.Create(ctx, ds); err != nil {
		return fmt.Errorf("create hostpath cleanup daemonset: %w", err)
	}

	if err := waitUntil(ctx, periodic.EventuallyTimeout, periodic.DefaultInterval, func() (bool, error) {
		var got appsv1.DaemonSet
		if err := k8sClient.Get(ctx, key, &got); err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}

			return false, fmt.Errorf("wait for hostpath cleanup daemonset ready: %w", err)
		}

		desired := got.Status.DesiredNumberScheduled
		if desired <= 0 || got.Status.NumberReady != desired {
			return false, nil
		}

		return true, nil
	}); err != nil {
		return err
	}

	if err := k8sClient.Delete(ctx, ds); err != nil {
		return fmt.Errorf("delete hostpath cleanup daemonset: %w", err)
	}

	// Deleting the DaemonSet triggers pod termination and the preStop hook (rm -rf) runs asynchronously.
	// TerminationGracePeriodSeconds (60s) gives the hook time to complete before the pod is killed.
	// We wait for the DaemonSet object itself to be gone, which implies all pods have been terminated.
	// Use 3×EventuallyTimeout (360s) because EventuallyTimeout (120s) does not give enough margin after
	// the 60s grace period on multi-node clusters where Kubernetes may take additional time to remove all pods.
	if err := waitUntil(ctx, 3*periodic.EventuallyTimeout, periodic.DefaultInterval, func() (bool, error) {
		err := k8sClient.Get(ctx, key, &appsv1.DaemonSet{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		if err != nil {
			return false, fmt.Errorf("wait for hostpath cleanup daemonset removed: %w", err)
		}

		return false, nil
	}); err != nil {
		return err
	}

	return nil
}

// waitUntil polls until done returns (true, nil), ctx is canceled, timeout elapses, or done returns a non-nil error.
func waitUntil(ctx context.Context, timeout, interval time.Duration, done func() (ok bool, err error)) error {
	deadline := time.Now().Add(timeout)

	for {
		ok, err := done()
		if err != nil {
			return err
		}

		if ok {
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s", timeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

func fluentBitHostPathCleanupDaemonSet(namespace string) *appsv1.DaemonSet {
	const volumeName = "hostpath"

	grace := int64(60)

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fluentBitHostPathCleanupDSName,
			Namespace: namespace,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": fluentBitHostPathCleanupDSName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": fluentBitHostPathCleanupDSName},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &grace,
					Containers: []corev1.Container{
						{
							Name:    "sleep",
							Image:   fluentBitHostPathCleanupImage,
							Command: []string{"sh", "-c", "sleep 3600"},
							VolumeMounts: []corev1.VolumeMount{
								{Name: volumeName, MountPath: fluentBitHostPathMountInPod},
							},
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.LifecycleHandler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh", "-c", "rm -rf " + fluentBitHostPathMountInPod + "/*"},
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
									Path: fluentBitHostPathOnNode,
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

func allRuntimeResourcesDeleted(ctx context.Context, k8sClient client.Client) func() error {
	ns := client.InNamespace(kymaSystemNamespace)

	return func() error {
		var deployments appsv1.DeploymentList
		if err := k8sClient.List(ctx, &deployments, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list deployments: %w", err)
		}

		if n := len(deployments.Items); n > 0 {
			return fmt.Errorf("%d deployment(s) still exist", n)
		}

		var daemonSets appsv1.DaemonSetList
		if err := k8sClient.List(ctx, &daemonSets, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list daemonsets: %w", err)
		}

		if n := len(daemonSets.Items); n > 0 {
			return fmt.Errorf("%d daemonset(s) still exist", n)
		}

		var configMaps corev1.ConfigMapList
		if err := k8sClient.List(ctx, &configMaps, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list configmaps: %w", err)
		}

		if n := len(configMaps.Items); n > 0 {
			return fmt.Errorf("%d configmap(s) still exist", n)
		}

		var secrets corev1.SecretList
		if err := k8sClient.List(ctx, &secrets, ns, runtimeResourceSelector); err != nil {
			return fmt.Errorf("failed to list secrets: %w", err)
		}

		if n := len(secrets.Items); n > 0 {
			return fmt.Errorf("%d secret(s) still exist", n)
		}

		return nil
	}
}
