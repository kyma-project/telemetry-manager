package telemetry

//
//import (
//	"context"
//	operatorv1alpha1 "github.com/kyma-project/telemetry-manager/apis/operator/v1alpha1"
//	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
//	"github.com/kyma-project/telemetry-manager/internal/reconciler"
//	"github.com/kyma-project/telemetry-manager/internal/reconciler/telemetry/mocks"
//	"github.com/stretchr/testify/mock"
//	"github.com/stretchr/testify/require"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"k8s.io/apimachinery/pkg/runtime"
//	"k8s.io/apimachinery/pkg/types"
//	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
//	"k8s.io/client-go/tools/record"
//	"sigs.k8s.io/controller-runtime/pkg/client/fake"
//	"testing"
//)
//
//func initReconciler() *Reconciler {
//	scheme := runtime.NewScheme()
//	_ = clientgoscheme.AddToScheme(scheme)
//	_ = telemetryv1alpha1.AddToScheme(scheme)
//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
//
//	lcCond := NewLogCollectorConditions(fakeClient, types.NamespacedName{Name: "telemetry-fluent-bit", Namespace: "default"})
//	mcCond := NewMetricCollector(fakeClient, types.NamespacedName{Name: "telemtry-metric-gateway", Namespace: "default"})
//	tcCond := NewTraceCollector(fakeClient, types.NamespacedName{Name: "telemetry-trace-collector", Namespace: "default"})
//	config := Config{
//		TraceConfig: TraceConfig{
//			ServiceName: "trace-otlp-svc",
//			Namespace:   "default",
//		},
//		MetricConfig: MetricConfig{
//			ServiceName: "metric-otlp-svc",
//			Namespace:   "default",
//		},
//		Webhook: WebhookConfig{Enabled: false},
//	}
//
//	return NewReconciler(fakeClient, scheme, record.NewFakeRecorder(100), config, lcCond, tcCond, mcCond)
//}
//
//func TestUpdateConditions(t *testing.T) {
//	ctx := context.Background()
//	obj := operatorv1alpha1.Telemetry{
//		TypeMeta:   metav1.TypeMeta{},
//		ObjectMeta: metav1.ObjectMeta{},
//		Spec:       operatorv1alpha1.TelemetrySpec{},
//		Status:     operatorv1alpha1.TelemetryStatus{},
//	}
//
//	proberStub := &mocks.componentHealthChecker{}
//	condition := metav1.Condition{
//		Type:               "Logging",
//		Status:             "True",
//		ObservedGeneration: 1,
//		Reason:             reconciler.ReasonNoPipelineDeployed,
//		Message:            reconciler.Conditions[reconciler.ReasonNoPipelineDeployed],
//	}
//	proberStub.On("check", mock.Anything, mock.Anything).Return(&condition, nil)
//
//	rc := initReconciler()
//	rc.updateConditions(ctx, proberStub, &obj)
//	conditions := obj.Status.Conditions
//	require.Len(t, conditions, 1)
//
//}
