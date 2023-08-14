package telemetry

//func TestAllPipelinesHealthy(t *testing.T) {
//	ctx := context.Background()
//	scheme := runtime.NewScheme()
//	_ = clientgoscheme.AddToScheme(scheme)
//	_ = telemetryv1alpha1.AddToScheme(scheme)
//	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
//	lc := NewLogCollectorConditions(fakeClient, kubernetes.DaemonSetProber{fakeClient}, types.NamespacedName{Namespace: "kyma-system", Name: "telemetry-fluent-bit"})
//
//}
