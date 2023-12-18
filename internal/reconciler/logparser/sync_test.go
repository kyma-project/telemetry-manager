package logparser

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils/mocks"
)

var (
	testConfig = Config{
		DaemonSet:        types.NamespacedName{Name: "test-telemetry-fluent-bit", Namespace: "default"},
		ParsersConfigMap: types.NamespacedName{Name: "test-telemetry-fluent-bit-parsers", Namespace: "default"},
	}
)

func TestSyncParsersConfigMapErrorClientErrorReturnsError(t *testing.T) {
	mockClient := &mocks.Client{}
	badReqErr := apierrors.NewBadRequest("")
	mockClient.On("Get", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
	mockClient.On("List", mock.Anything, mock.Anything, mock.Anything).Return(badReqErr)
	sut := syncer{mockClient, testConfig}

	err := sut.syncFluentBitConfig(context.Background())

	require.Error(t, err)
}

func TestSuccessfulParserConfigMap(t *testing.T) {
	var ctx context.Context

	s := clientgoscheme.Scheme
	err := telemetryv1alpha1.AddToScheme(clientgoscheme.Scheme)
	require.NoError(t, err)

	lp := &telemetryv1alpha1.LogParser{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},
		Spec: telemetryv1alpha1.LogParserSpec{Parser: `
Format regex`},
	}
	mockClient := fake.NewClientBuilder().WithScheme(s).WithObjects(lp).Build()
	sut := syncer{mockClient, testConfig}

	err = sut.syncFluentBitConfig(context.Background())

	require.NoError(t, err)

	var cm corev1.ConfigMap
	err = sut.Get(ctx, testConfig.ParsersConfigMap, &cm)
	require.NoError(t, err)
	expectedCMData := "[PARSER]\n    Name foo\n    Format regex\n\n"
	require.Contains(t, cm.Data[parsersConfigMapKey], expectedCMData)
	require.Len(t, cm.OwnerReferences, 1)
	require.Equal(t, lp.Name, cm.OwnerReferences[0].Name)
}
