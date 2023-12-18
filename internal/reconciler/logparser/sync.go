package logparser

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	telemetryv1alpha1 "github.com/kyma-project/telemetry-manager/apis/telemetry/v1alpha1"
	"github.com/kyma-project/telemetry-manager/internal/fluentbit/config/builder"
	"github.com/kyma-project/telemetry-manager/internal/k8sutils"
)

const parsersConfigMapKey = "parsers.conf"

type syncer struct {
	client.Client
	config Config
}

func (s *syncer) syncFluentBitConfig(ctx context.Context) error {
	cm, err := k8sutils.GetOrCreateConfigMap(ctx, s, s.config.ParsersConfigMap)
	if err != nil {
		return fmt.Errorf("unable to get parsers configmap: %w", err)
	}

	var logParsers telemetryv1alpha1.LogParserList

	err = s.List(ctx, &logParsers)
	if err != nil {
		return fmt.Errorf("unable to list parsers: %w", err)
	}
	fluentBitParsersConfig := builder.BuildFluentBitParsersConfig(&logParsers)
	if fluentBitParsersConfig == "" {
		data := make(map[string]string)
		data[parsersConfigMapKey] = ""
		cm.Data = data
	} else if cm.Data == nil {
		data := make(map[string]string)
		data[parsersConfigMapKey] = fluentBitParsersConfig
		cm.Data = data
	} else {
		if oldConfig, hasKey := cm.Data[parsersConfigMapKey]; !hasKey || oldConfig != fluentBitParsersConfig {
			cm.Data[parsersConfigMapKey] = fluentBitParsersConfig
		}
	}

	for i := range logParsers.Items {
		if err := controllerutil.SetOwnerReference(&logParsers.Items[i], &cm, s.Scheme()); err != nil {
			return fmt.Errorf("unable to set owner reference for parsers configmap: %w", err)
		}
	}

	if err = s.Update(ctx, &cm); err != nil {
		return fmt.Errorf("unable to parsers files configmap: %w", err)
	}

	return nil
}
