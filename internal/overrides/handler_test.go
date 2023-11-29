package overrides

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const conf = `
	global:
	  logLevel: info
	tracing:
	  paused: true
	`

func setup(withConfigMap bool) (client.WithWatch, zap.AtomicLevel, HandlerConfig) {
	const configMapKey = "override-config"
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "telemetry-system"},
		Data:       map[string]string{configMapKey: conf},
	}
	var fakeClient client.WithWatch
	if withConfigMap {
		fakeClient = fake.NewClientBuilder().WithObjects(configMap).Build()
	} else {
		fakeClient = fake.NewClientBuilder().Build()
	}
	level, _ := zapcore.ParseLevel("debug")
	atomicLevel := zap.NewAtomicLevelAt(level)
	handlerConfig := HandlerConfig{
		ConfigMapName: types.NamespacedName{Name: "foo", Namespace: "telemetry-system"},
		ConfigMapKey:  configMapKey,
	}

	return fakeClient, atomicLevel, handlerConfig
}

func TestConfigMapProber(t *testing.T) {
	fakeClient, atomicLevel, handlerConfig := setup(true)

	handler := New(fakeClient, atomicLevel, handlerConfig)
	cm, err := handler.readConfigMapOrEmpty(context.Background())

	require.NoError(t, err)
	require.Equal(t, conf, cm)
}

func TestConfigMapNotExist(t *testing.T) {
	fakeClient, atomicLevel, handlerConfig := setup(false)

	handler := New(fakeClient, atomicLevel, handlerConfig)
	cm, err := handler.readConfigMapOrEmpty(context.Background())

	require.NoError(t, err)
	require.Equal(t, "", cm)
}
