package labelupdater

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const testNamespace = "kyma-system"

func TestStart(t *testing.T) {
	tests := []struct {
		name              string
		existingObjects   []runtime.Object
		expectedLabelsSA  map[string]string
		expectedLabelsCRB map[string]string
	}{
		{
			name: "patches label on resources without it",
			existingObjects: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{Name: names.FluentBit, Namespace: testNamespace},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: names.FluentBit},
				},
			},
			expectedLabelsSA: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
			},
			expectedLabelsCRB: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
			},
		},
		{
			name: "preserves existing labels",
			existingObjects: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.FluentBit,
						Namespace: testNamespace,
						Labels:    map[string]string{"existing-label": "existing-value"},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{Name: names.FluentBit},
				},
			},
			expectedLabelsSA: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
				"existing-label":                   "existing-value",
			},
			expectedLabelsCRB: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
			},
		},
		{
			name: "skips resources already labeled",
			existingObjects: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      names.FluentBit,
						Namespace: testNamespace,
						Labels:    map[string]string{commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule},
					},
				},
				&rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   names.FluentBit,
						Labels: map[string]string{commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule},
					},
				},
			},
			expectedLabelsSA: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
			},
			expectedLabelsCRB: map[string]string{
				commonresources.LabelKeyKymaModule: commonresources.LabelValueKymaModule,
			},
		},
		{
			name:              "succeeds when resources do not exist",
			existingObjects:   nil,
			expectedLabelsSA:  nil,
			expectedLabelsCRB: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := newFakeClient(t, tt.existingObjects...)
			updater := New(fakeClient, fakeClient, logr.Discard(), testNamespace)

			require.NoError(t, updater.Start(t.Context()))

			if tt.expectedLabelsSA != nil {
				var sa corev1.ServiceAccount
				require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: names.FluentBit, Namespace: testNamespace}, &sa))
				assert.Equal(t, tt.expectedLabelsSA, sa.Labels)
			}

			if tt.expectedLabelsCRB != nil {
				var crb rbacv1.ClusterRoleBinding
				require.NoError(t, fakeClient.Get(t.Context(), types.NamespacedName{Name: names.FluentBit}, &crb))
				assert.Equal(t, tt.expectedLabelsCRB, crb.Labels)
			}
		})
	}
}

func newFakeClient(t *testing.T, objs ...runtime.Object) client.Client {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRuntimeObjects(objs...).
		Build()
}
