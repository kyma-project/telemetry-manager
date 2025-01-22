package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterInfo struct {
	ClusterName   string
	CloudProvider string
}

func GetGardenerShootInfo(ctx context.Context, client client.Client) ClusterInfo {
	shootInfo := corev1.ConfigMap{}
	err := client.Get(ctx, types.NamespacedName{
		Namespace: "kube-system",
		Name:      "shoot-info",
	}, &shootInfo)

	if err != nil {
		logf.FromContext(ctx).V(1).Info("Failed get shoot-info config map")
		return ClusterInfo{ClusterName: "${KUBERNETES_SERVICE_HOST}"}
	}

	return ClusterInfo{
		ClusterName:   shootInfo.Data["shootName"],
		CloudProvider: shootInfo.Data["provider"],
	}
}
