package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultClusterName                 = "${KUBERNETES_SERVICE_HOST}"
	gardenerCloudProviderAttributeName = "provider"
	CloudProviderOpenStack             = "openstack"
	CloudProviderSAPConvergedCloud     = "sap"
)

var defaultGardenerShootInfoCM = types.NamespacedName{
	Namespace: "kube-system",
	Name:      "shoot-info",
}

type ClusterInfo struct {
	ClusterName   string
	CloudProvider string
}

func GetGardenerShootInfo(ctx context.Context, client client.Client) ClusterInfo {
	shootInfo := corev1.ConfigMap{}

	err := client.Get(ctx, defaultGardenerShootInfoCM, &shootInfo)

	// The shoot-info config map is used to store information about the Gardener cluster, such as the cluster name and the cloud provider.
	// If cluster in a non Gardener cluster, the shoot-info config map will not exist, so we return the default values.
	if err != nil {
		logf.FromContext(ctx).V(1).Info("Failed get shoot-info config map")

		return ClusterInfo{ClusterName: defaultClusterName}
	}

	// The provider `openstack` is used to represent the SAP Converged Cloud provider.
	cloudProvider := shootInfo.Data[gardenerCloudProviderAttributeName]

	if cloudProvider == CloudProviderOpenStack {
		cloudProvider = CloudProviderSAPConvergedCloud
	}

	return ClusterInfo{
		CloudProvider: cloudProvider,
		ClusterName:   defaultClusterName,
	}
}

// GetClusterUID retrieves the unique identifier of the Kubernetes cluster by fetching the UID of the kube-system namespace.
func GetClusterUID(ctx context.Context, client client.Client) (string, error) {
	var kubeSystem corev1.Namespace

	kubeSystemNs := types.NamespacedName{
		Name: "kube-system",
	}

	err := client.Get(ctx, kubeSystemNs, &kubeSystem)
	if err != nil {
		return "", err
	}

	return string(kubeSystem.UID), nil
}
