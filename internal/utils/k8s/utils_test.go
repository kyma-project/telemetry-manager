package k8s

import (
	"testing"

	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMergePodAnnotations(t *testing.T) {
	tests := []struct {
		name           string
		existing       map[string]string
		desired        map[string]string
		expectedMerged map[string]string
	}{
		{
			name: "should preserve existing kubectl annotations",
			existing: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "2",
				"kubectl.kubernetes.io/3": "3",
				"unrelated":               "foo",
			},
			desired: map[string]string{
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
			expectedMerged: map[string]string{
				"kubectl.kubernetes.io/1": "1",
				"kubectl.kubernetes.io/2": "b",
				"kubectl.kubernetes.io/3": "3",
				"kubectl.kubernetes.io/4": "4",
			},
		},
		{
			name: "should preserve existing checksum annotations",
			existing: map[string]string{
				"checksum/config":   "1",
				"checksum/config-a": "2",
				"checksum/Config":   "3",
				"unrelated":         "foo",
			},
			desired: map[string]string{
				"checksum/config-a": "5",
				"checksum/cOnfig":   "6",
			},
			expectedMerged: map[string]string{
				"checksum/config":   "1",
				"checksum/config-a": "5",
				"checksum/cOnfig":   "6",
			},
		},
		{
			name: "should preserve existing istio restartedAt annotations",
			existing: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"unrelated": "foo",
			},
			desired: map[string]string{
				"checksum/1": "1",
			},
			expectedMerged: map[string]string{
				"istio-operator.kyma-project.io/restartedAt": "2023-08-17T13:36:09Z",
				"checksum/1": "1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-deployment",
					Annotations: tt.existing,
				},
			}

			desired := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "some-deployment",
					Annotations: tt.desired,
				},
			}

			mergePodAnnotations(&desired.ObjectMeta, existing.ObjectMeta)

			require.Equal(t, len(tt.expectedMerged), len(desired.Annotations))

			for k, v := range tt.expectedMerged {
				require.Contains(t, desired.Annotations, k)
				require.Equal(t, v, desired.Annotations[k])
			}
		})
	}
}

func TestMergeOwnerReference(t *testing.T) {
	oldOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-1",
			UID:        "old-deployment-uid-1",
		},
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "old-deployment-2",
			UID:        "old-deployment-uid-2",
		},
	}
	newOwners := []metav1.OwnerReference{
		{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       "new-deployment-1",
			UID:        "new-deployment-uid-1",
		},
	}

	merged := mergeOwnerReferences(newOwners, oldOwners)
	require.Equal(t, 3, len(merged))
}
