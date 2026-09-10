package logpipelinelockmigration

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1beta1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1beta1"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
)

const testNamespace = "kyma-system"

func TestCleanupLegacyLock(t *testing.T) {
	otelPipeline := newLogPipeline("otel-pipeline", "uid-otel", true)
	fluentBitPipeline := newLogPipeline("fluentbit-pipeline", "uid-fluentbit", false)

	tests := []struct {
		name         string
		pipelines    []client.Object
		lockOwners   []metav1.OwnerReference
		lockExists   bool
		expectDelete bool
	}{
		{
			name:         "no legacy lock present",
			lockExists:   false,
			expectDelete: false,
		},
		{
			name:         "lock has no owners",
			lockExists:   true,
			lockOwners:   nil,
			expectDelete: false,
		},
		{
			name:         "lock has only OTel owners",
			pipelines:    []client.Object{otelPipeline},
			lockExists:   true,
			lockOwners:   []metav1.OwnerReference{ownerRefFor(otelPipeline)},
			expectDelete: false,
		},
		{
			name:         "lock has a FluentBit owner",
			pipelines:    []client.Object{otelPipeline, fluentBitPipeline},
			lockExists:   true,
			lockOwners:   []metav1.OwnerReference{ownerRefFor(otelPipeline), ownerRefFor(fluentBitPipeline)},
			expectDelete: true,
		},
		{
			name:         "lock has an owner ref to a nonexistent pipeline",
			pipelines:    []client.Object{otelPipeline},
			lockExists:   true,
			lockOwners:   []metav1.OwnerReference{ownerRefFor(otelPipeline), ownerRefForMissing()},
			expectDelete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := newTestScheme(t)

			objects := append([]client.Object{}, tt.pipelines...)
			if tt.lockExists {
				objects = append(objects, newLock(tt.lockOwners))
			}

			fakeClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(objects...).
				Build()

			migrator := New(fakeClient, logr.Discard(), testNamespace)

			err := migrator.cleanupOldLockIfNeeded(context.Background())
			require.NoError(t, err)

			var lock corev1.ConfigMap
			getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: names.LogPipelineLock, Namespace: testNamespace}, &lock)

			if tt.expectDelete {
				require.True(t, apierrors.IsNotFound(getErr), "expected lock to be deleted")
				return
			}

			if tt.lockExists {
				require.NoError(t, getErr, "expected lock to still exist")
			}
		})
	}
}

// TestCleanupLegacyLock_Idempotent verifies that a second pass after the lock
// has been rebuilt cleanly (only OTel owners) does not delete it again. This is
// the fixpoint that makes the retry loop safe.
func TestCleanupLegacyLock_Idempotent(t *testing.T) {
	scheme := newTestScheme(t)

	otelPipeline := newLogPipeline("otel-pipeline", "uid-otel", true)
	cleanLock := newLock([]metav1.OwnerReference{ownerRefFor(otelPipeline)})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(otelPipeline, cleanLock).
		Build()

	migrator := New(fakeClient, logr.Discard(), testNamespace)

	require.NoError(t, migrator.cleanupOldLockIfNeeded(context.Background()))

	var lock corev1.ConfigMap
	require.NoError(t, fakeClient.Get(context.Background(), types.NamespacedName{Name: names.LogPipelineLock, Namespace: testNamespace}, &lock))
}

// TestStart verifies that a single successful cleanup pass makes Start return
// nil rather than looping.
func TestStart(t *testing.T) {
	scheme := newTestScheme(t)

	fluentBitPipeline := newLogPipeline("fluentbit-pipeline", "uid-fluentbit", false)
	contaminatedLock := newLock([]metav1.OwnerReference{ownerRefFor(fluentBitPipeline)})

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(fluentBitPipeline, contaminatedLock).
		Build()

	migrator := New(fakeClient, logr.Discard(), testNamespace)

	require.NoError(t, migrator.Start(context.Background()))

	var lock corev1.ConfigMap
	getErr := fakeClient.Get(context.Background(), types.NamespacedName{Name: names.LogPipelineLock, Namespace: testNamespace}, &lock)
	require.True(t, apierrors.IsNotFound(getErr), "expected contaminated lock to be deleted")
}

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, telemetryv1beta1.AddToScheme(scheme))

	return scheme
}

// newLogPipeline builds a LogPipeline. When otel is true the pipeline uses an
// OTLP output (OTel mode); otherwise it uses a FluentBit custom output.
func newLogPipeline(name string, uid types.UID, otel bool) *telemetryv1beta1.LogPipeline {
	output := telemetryv1beta1.LogPipelineOutput{}
	if otel {
		output.OTLP = &telemetryv1beta1.OTLPOutput{
			Endpoint: telemetryv1beta1.ValueType{Value: "http://example.com"},
		}
	} else {
		output.FluentBitCustom = "Name stdout"
	}

	return &telemetryv1beta1.LogPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			UID:  uid,
		},
		Spec: telemetryv1beta1.LogPipelineSpec{
			Output: output,
		},
	}
}

func newLock(owners []metav1.OwnerReference) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            names.LogPipelineLock,
			Namespace:       testNamespace,
			OwnerReferences: owners,
		},
	}
}

func ownerRefFor(lp *telemetryv1beta1.LogPipeline) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: telemetryv1beta1.GroupVersion.String(),
		Kind:       "LogPipeline",
		Name:       lp.Name,
		UID:        lp.UID,
	}
}

func ownerRefForMissing() metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: telemetryv1beta1.GroupVersion.String(),
		Kind:       "LogPipeline",
		Name:       "deleted-pipeline",
		UID:        "uid-deleted",
	}
}
