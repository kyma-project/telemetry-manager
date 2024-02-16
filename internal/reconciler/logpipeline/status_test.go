package logpipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/conditions"
	"github.com/kyma-project/telemetry-manager/internal/reconciler/logpipeline/mocks"
)

func TestUpdateStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = telemetryv1alpha1.AddToScheme(scheme)

	t.Run("should add pending condition if some referenced secret does not exist", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "host",
								},
							},
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: &mocks.DaemonSetProber{},
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonReferencedSecretMissing), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add pending condition if referenced secret exists but fluent bit is not ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "host",
								},
							},
						},
					},
				}},
		}
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add pending condition if Loki output is defined", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Loki: &telemetryv1alpha1.LokiOutput{
						URL: telemetryv1alpha1.ValueType{
							Value: "http://logging-loki:3100/loki/api/v1/push",
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonUnsupportedLokiOutput), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should add running condition if referenced secret exists and fluent bit is ready", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "host",
								},
							},
						},
					},
				}},
		}
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypeRunning, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if fluent bit becomes not ready again", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "host",
								},
							},
						},
					},
				}},
		}
		secret := &corev1.Secret{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-secret",
				Namespace: "some-namespace",
			},
			Data: map[string][]byte{"host": nil},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline, secret).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(false, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonFluentBitDSNotReady, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if some referenced secret does not exist anymore", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					HTTP: &telemetryv1alpha1.HTTPOutput{
						Host: telemetryv1alpha1.ValueType{
							ValueFrom: &telemetryv1alpha1.ValueFromSource{
								SecretKeyRef: &telemetryv1alpha1.SecretKeyRef{
									Name:      "some-secret",
									Namespace: "some-namespace",
									Key:       "host",
								},
							},
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: &mocks.DaemonSetProber{},
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonReferencedSecretMissing, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonReferencedSecretMissing), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should remove running condition and set pending condition to true if Loki output is defined", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:       pipelineName,
				Generation: 1,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				Conditions: []metav1.Condition{
					{
						Type:               conditions.TypePending,
						Status:             metav1.ConditionFalse,
						Reason:             conditions.ReasonFluentBitDSNotReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSNotReady),
						LastTransitionTime: metav1.Now(),
					},
					{
						Type:               conditions.TypeRunning,
						Status:             metav1.ConditionTrue,
						Reason:             conditions.ReasonFluentBitDSReady,
						Message:            conditions.CommonMessageFor(conditions.ReasonFluentBitDSReady),
						LastTransitionTime: metav1.Now(),
					},
				},
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Output: telemetryv1alpha1.Output{
					Loki: &telemetryv1alpha1.LokiOutput{
						URL: telemetryv1alpha1.ValueType{
							Value: "http://logging-loki:3100/loki/api/v1/push",
						},
					},
				}},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()

		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)

		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)
		require.Len(t, updatedPipeline.Status.Conditions, 1)
		require.Equal(t, conditions.TypePending, updatedPipeline.Status.Conditions[0].Type)
		require.Equal(t, metav1.ConditionTrue, updatedPipeline.Status.Conditions[0].Status)
		require.Equal(t, conditions.ReasonUnsupportedLokiOutput, updatedPipeline.Status.Conditions[0].Reason)
		require.Equal(t, conditions.CommonMessageFor(conditions.ReasonUnsupportedLokiOutput), updatedPipeline.Status.Conditions[0].Message)
		require.Equal(t, updatedPipeline.Generation, updatedPipeline.Status.Conditions[0].ObservedGeneration)
		require.NotEmpty(t, updatedPipeline.Status.Conditions[0].LastTransitionTime)
	})

	t.Run("should set status UnsupportedMode true if contains custom plugin", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				UnsupportedMode: false,
			},
			Spec: telemetryv1alpha1.LogPipelineSpec{
				Filters: []telemetryv1alpha1.Filter{{Custom: "some-filter"}},
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		require.True(t, updatedPipeline.Status.UnsupportedMode)
	})

	t.Run("should set status UnsupportedMode false if does not contains custom plugin", func(t *testing.T) {
		pipelineName := "pipeline"
		pipeline := &telemetryv1alpha1.LogPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: pipelineName,
			},
			Status: telemetryv1alpha1.LogPipelineStatus{
				UnsupportedMode: true,
			},
		}
		fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(pipeline).WithStatusSubresource(pipeline).Build()
		proberStub := &mocks.DaemonSetProber{}
		proberStub.On("IsReady", mock.Anything, mock.Anything).Return(true, nil)
		sut := Reconciler{
			Client: fakeClient,
			config: Config{DaemonSet: types.NamespacedName{Name: "fluent-bit"}},
			prober: proberStub,
		}

		err := sut.updateStatus(context.Background(), pipeline.Name)
		require.NoError(t, err)

		var updatedPipeline telemetryv1alpha1.LogPipeline
		_ = fakeClient.Get(context.Background(), types.NamespacedName{Name: pipelineName}, &updatedPipeline)

		require.False(t, updatedPipeline.Status.UnsupportedMode)
	})
}
