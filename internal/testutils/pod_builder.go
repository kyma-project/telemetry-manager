package testutils

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type podBuilder struct {
	name                 string
	namespace            string
	labels               map[string]string
	status               *corev1.PodStatus
	withExpiredThreshold bool
}

func NewPodBuilder(name, namespace string) *podBuilder {
	pb := &podBuilder{
		name:      name,
		namespace: namespace,
	}
	return pb
}

func (pb *podBuilder) WithLabels(labels map[string]string) *podBuilder {
	pb.labels = labels
	return pb
}

func (pb *podBuilder) WithExpiredThreshold() *podBuilder {
	pb.withExpiredThreshold = true
	return pb
}

func (pb *podBuilder) WithImageNotFound() *podBuilder {
	pb.status = &corev1.PodStatus{
		Phase: corev1.PodRunning,
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

func (pb *podBuilder) WithOOMStatus() *podBuilder {
	pb.status = &corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "collector",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "back-off 5m0s restarting failed container=collector pod=telemetry-trace-collector-7794f88496-h5ntd_kyma-system(f229ae7c-dbc9-4642-ba34-4f74f40df390)",
					},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:   137,
						Signal:     0,
						Reason:     "OOMKilled",
						StartedAt:  metav1.NewTime(time.Now()),
						FinishedAt: metav1.NewTime(time.Now()),
					}},
			},
		},
	}
	return pb
}

func (pb *podBuilder) WithCrashBackOffStatus() *podBuilder {

	pb.status = &corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "collector",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "CrashLoopBackOff",
						Message: "back-off 1m20s restarting failed container=collector pod=telemetry-trace-collector-757759865f-wnx7h_kyma-system(9525d87f-7e91-402c-995d-6f64fda1c2c6)",
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

func (pb *podBuilder) WithEvictedStatus() *podBuilder {
	pb.status = &corev1.PodStatus{
		Phase:   corev1.PodFailed,
		Reason:  "Evicted",
		Message: "The node was low on resource: memory. Container collector was using 100Mi, which exceeds its request of 0.",
	}
	return pb
}

func (pb *podBuilder) WithPendingStatus() *podBuilder {
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

func (pb *podBuilder) WithNonZeroExitStatus() *podBuilder {
	pb.status = &corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{
			{
				Name: "collector",
				State: corev1.ContainerState{
					Waiting: &corev1.ContainerStateWaiting{
						Reason:  "RunContainerError",
						Message: "'failed to start containerd task",
					},
				},
				LastTerminationState: corev1.ContainerState{
					Terminated: &corev1.ContainerStateTerminated{
						ExitCode:   2,
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

func (pb *podBuilder) WithRunningStatus() *podBuilder {
	pb.status = &corev1.PodStatus{
		Phase: corev1.PodRunning,
	}
	return pb
}
func (pb *podBuilder) Build() *corev1.Pod {
	if pb.labels == nil {
		pb.labels = make(map[string]string)
		pb.labels["app"] = "foo"
	}
	pod := &corev1.Pod{
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

	if len(pod.Status.Conditions) != 0 && pb.withExpiredThreshold {
		pod.Status.Conditions[0].LastTransitionTime = metav1.NewTime(time.Now().Add(-1 * time.Hour))

	}

	if len(pod.Status.ContainerStatuses) != 0 && pb.withExpiredThreshold {
		pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.StartedAt = metav1.NewTime(time.Now().Add(-1 * time.Hour))
	}
	return pod
}

//func CreatePodList(ready int32, labels map[string]string) []*corev1.Pod {
//	var pods []*corev1.Pod
//	for i := 0; i < int(ready); i++ {
//		name := fmt.Sprintf("pod-%d", i)
//		pod := NewPodBuilder(name, "telemetry-system").WithLabels(labels).Build()
//		pods = append(pods, pod)
//	}
//	return pods
//}
