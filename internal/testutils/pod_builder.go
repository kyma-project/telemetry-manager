package testutils

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodBuilder struct {
	name                 string
	namespace            string
	labels               map[string]string
	status               *corev1.PodStatus
	withExpiredThreshold bool
}

func NewPodBuilder(name, namespace string) *PodBuilder {
	pb := &PodBuilder{
		name:      name,
		namespace: namespace,
	}
	return pb
}

func (pb *PodBuilder) WithLabels(labels map[string]string) *PodBuilder {
	pb.labels = labels
	return pb
}

func (pb *PodBuilder) WithImageNotFound() *PodBuilder {

	pb.status = &corev1.PodStatus{
		Phase:      corev1.PodPending,
		Conditions: createPodReadyConditions(corev1.ConditionFalse),
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "collector",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "ImagePullBackOff",
						Message: "Back-off pulling image \"foo:bar\"",
					},
				},
			},
		},
	}
	return pb
}

func (pb *PodBuilder) WithOOMStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase:             corev1.PodRunning,
		Conditions:        createPodReadyConditions(corev1.ConditionFalse),
		ContainerStatuses: createContainerStatus("OOMKilled", "Container was OOM killed", "OOMKilled", 137),
	}
	return pb
}

func (pb *PodBuilder) WithCrashBackOffStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: createPodReadyConditions(corev1.ConditionFalse),
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "collector",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "back-off 1m20s restarting failed container=collector pod=telemetry-trace-gateway-757759865f-wnx7h_kyma-system(9525d87f-7e91-402c-995d-6f64fda1c2c6)",
					},
				},
				LastTerminationState: corev1.ContainerState{
					Waiting: nil,
					Running: nil,
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:   1,
						Signal:     0,
						Reason:     "Error",
						StartedAt:  metav1.NewTime(time.Now()),
						FinishedAt: metav1.NewTime(time.Now()),
					}},
			},
		},
	}
	return pb
}

func (pb *PodBuilder) WithEvictedStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase:   corev1.PodFailed,
		Reason:  "Evicted",
		Message: "The node was low on resource: memory. Container collector was using 100Mi, which exceeds its request of 0.",
	}
	return pb
}

func (pb *PodBuilder) WithPendingStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase: corev1.PodPending,
		Conditions: []corev1.PodCondition{
			{
				Type:               corev1.PodScheduled,
				Status:             corev1.ConditionFalse,
				LastTransitionTime: metav1.NewTime(time.Now()),
				Message:            "Unschedulable Message:0/2 nodes are available: persistentvolumeclaim \"my-pvc\" not found. preemption: 0/2 nodes are available: 2 No preemption victims found for incoming pod.",
			},
		},
	}
	return pb
}

func (pb *PodBuilder) WithNonZeroExitStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase:             corev1.PodRunning,
		Conditions:        createPodReadyConditions(corev1.ConditionFalse),
		ContainerStatuses: createContainerStatus("Error", "Container failed", "Error", 2),
	}
	return pb
}

func (pb *PodBuilder) WithRunningStatus() *PodBuilder {
	pb.status = &corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: createPodReadyConditions(corev1.ConditionTrue),
	}
	return pb
}
func (pb *PodBuilder) Build() corev1.Pod {
	if pb.labels == nil {
		pb.labels = make(map[string]string)
		pb.labels["app"] = "foo"
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.name,
			Namespace: pb.namespace,
			Labels:    pb.labels,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "foo",
					Image: "foo",
				},
			},
		},
	}

	if pb.status != nil {
		pod.Status = *pb.status
	}

	if len(pod.Status.ContainerStatuses) != 0 && pb.withExpiredThreshold {
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.StartedAt = metav1.NewTime(time.Now().Add(-1 * time.Hour))
	}
	return pod
}

func createContainerStatus(waitingReason, waitingMsg, terminatedReason string, exitCode int32) []corev1.ContainerStatus {
	return []corev1.ContainerStatus{
		{
			Name: "collector",
			State: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{
					Reason:  waitingReason,
					Message: waitingMsg,
				},
			},
			LastTerminationState: corev1.ContainerState{
				Terminated: &corev1.ContainerStateTerminated{
					ExitCode:   exitCode,
					Signal:     0,
					Reason:     terminatedReason,
					StartedAt:  metav1.NewTime(time.Now()),
					FinishedAt: metav1.NewTime(time.Now()),
				}},
		},
	}
}

func createPodReadyConditions(status corev1.ConditionStatus) []corev1.PodCondition {
	condition := corev1.PodCondition{
		Type:               corev1.PodReady,
		Status:             status,
		LastProbeTime:      metav1.Time{},
		LastTransitionTime: metav1.NewTime(time.Now()),
		Reason:             "",
		Message:            "",
	}
	conditions := []corev1.PodCondition{}
	conditions = append(conditions, condition)
	return conditions
}
