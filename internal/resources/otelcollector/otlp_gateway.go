package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"istio.io/api/networking/v1alpha3"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

// OTLPGatewayApplierDeleter manages the unified OTLP gateway deployed as a DaemonSet.
// It wraps a GatewayApplierDeleter and adds logic to handle migration from the old log gateway deployment.
type OTLPGatewayApplierDeleter struct {
	globals config.Global

	baseName        string
	extraPodLabels  map[string]string
	image           string
	otlpServiceName string
	rbac            rbac

	baseMemoryLimit      resource.Quantity
	dynamicMemoryLimit   resource.Quantity
	baseCPURequest       resource.Quantity
	dynamicCPURequest    resource.Quantity
	baseMemoryRequest    resource.Quantity
	dynamicMemoryRequest resource.Quantity

	podOpts       []commonresources.PodSpecOption
	containerOpts []commonresources.ContainerOption
}

// NewOTLPGatewayApplierDeleter creates a new OTLPGatewayApplierDeleter that manages the OTLP gateway DaemonSet.
//
//nolint:dupl // repeating the code as we this would be deleted when we implement all signals in OTLP gateway
func NewOTLPGatewayApplierDeleter(globals config.Global, image, priorityClassName string) *OTLPGatewayApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryLogIngest: commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryLogExport: commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:        commonresources.LabelValueTrue, // inject istio sidecar
	}

	return &OTLPGatewayApplierDeleter{
		globals:              globals,
		baseName:             names.OTLPGateway,
		extraPodLabels:       extraLabels,
		image:                image,
		otlpServiceName:      names.OTLPService,
		rbac:                 makeOTLPGatewayRBAC(globals.TargetNamespace()),
		baseMemoryLimit:      logGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   logGatewayDynamicMemoryLimit,
		baseCPURequest:       logGatewayBaseCPURequest,
		dynamicCPURequest:    logGatewayDynamicCPURequest,
		baseMemoryRequest:    logGatewayBaseMemoryRequest,
		dynamicMemoryRequest: logGatewayDynamicMemoryRequest,
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithAffinity(makePodAffinity(commonresources.MakeDefaultSelectorLabels(names.OTLPGateway))),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
		},
	}
}

// ApplyResources creates or updates the OTLP gateway DaemonSet and Legacy otlp logs service.
func (o *OTLPGatewayApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error {
	name := types.NamespacedName{Namespace: o.globals.TargetNamespace(), Name: o.baseName}

	ingressAllowedPorts := gatewayIngressAllowedPorts()
	if opts.IstioEnabled {
		ingressAllowedPorts = append(ingressAllowedPorts, ports.IstioEnvoy)
	}

	if err := applyCommonResources(ctx, c, name, commonresources.LabelValueK8sComponentGateway, o.rbac, ingressAllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secret := makeSecret(name, commonresources.LabelValueK8sComponentGateway, opts.CollectorEnvVars)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, commonresources.LabelValueK8sComponentGateway, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, c, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})

	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, c, o.makeGatewayDaemonSet(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, o.makeOTLPService()); err != nil {
		return fmt.Errorf("failed to create otlp service: %w", err)
	}

	// Create the legacy service for backward compatibility
	// This service uses the old name (telemetry-otlp-logs) but points to the new DaemonSet
	legacyService := o.makeLegacyOTLPService(names.OTLPLogsService)
	if err := k8sutils.CreateOrUpdateService(ctx, c, legacyService); err != nil {
		return fmt.Errorf("failed to create legacy log otlp service: %w", err)
	}

	if opts.IstioEnabled {
		for _, svcName := range []string{names.OTLPLogsService, names.OTLPService} {
			if err := k8sutils.CreateOrUpdateDestinationRule(ctx, c, o.makeDestinationRule(svcName)); err != nil {
				return fmt.Errorf("failed to create destinationrule: %w", err)
			}
		}
	}

	return nil
}

// DeleteResources removes all OTLP gateway resources.
func (o *OTLPGatewayApplierDeleter) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: o.baseName, Namespace: o.globals.TargetNamespace()}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      o.baseName,
		Namespace: o.globals.TargetNamespace(),
	}

	secret := corev1.Secret{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &secret); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete env secret: %w", err))
	}

	configMap := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &configMap); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete configmap: %w", err))
	}

	daemonSet := appsv1.DaemonSet{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &daemonSet); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete daemonset: %w", err))
	}

	// Delete the OTLP service
	OTLPService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: o.otlpServiceName, Namespace: o.globals.TargetNamespace()}}
	if err := k8sutils.DeleteObject(ctx, c, &OTLPService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete otlp service: %w", err))
	}

	legacyLogService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.OTLPLogsService, Namespace: o.globals.TargetNamespace()}}
	if err := k8sutils.DeleteObject(ctx, c, &legacyLogService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy log otlp service: %w", err))
	}

	if isIstioActive {
		for _, svcName := range []string{names.OTLPLogsService, names.OTLPService} {
			destinationRuleMeta := metav1.ObjectMeta{Namespace: o.globals.TargetNamespace(), Name: svcName}

			destinationRule := istionetworkingclientv1.DestinationRule{ObjectMeta: destinationRuleMeta}
			if err := k8sutils.DeleteObject(ctx, c, &destinationRule); err != nil {
				allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete destinationrule: %w", err))
			}
		}
	}

	return allErrors
}

func (o *OTLPGatewayApplierDeleter) makeDestinationRule(name string) *istionetworkingclientv1.DestinationRule {
	commonLabels := commonresources.MakeDefaultLabels(o.baseName, commonresources.LabelValueK8sComponentGateway)

	return &istionetworkingclientv1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.globals.TargetNamespace(),
			Labels:    commonLabels,
		},
		Spec: v1alpha3.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", name, o.globals.TargetNamespace()),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				Tls: &v1alpha3.ClientTLSSettings{Mode: v1alpha3.ClientTLSSettings_DISABLE},
			},
		},
	}
}

func (o *OTLPGatewayApplierDeleter) makeOTLPService() *corev1.Service {
	commonLabels := commonresources.MakeDefaultLabels(o.baseName, commonresources.LabelValueK8sComponentGateway)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(o.baseName)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.otlpServiceName,
			Namespace: o.globals.TargetNamespace(),
			Labels:    commonLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPGRPC,
					TargetPort: intstr.FromInt32(ports.OTLPGRPC),
				},
				{
					Name:       "http-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPHTTP,
					TargetPort: intstr.FromInt32(ports.OTLPHTTP),
				},
			},
			Selector:              selectorLabels,
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
		},
	}

	return service
}

// makeLegacyOTLPService creates a service with a legacy name that points to the unified OTLP gateway
func (o *OTLPGatewayApplierDeleter) makeLegacyOTLPService(legacyServiceName string) *corev1.Service {
	commonLabels := commonresources.MakeDefaultLabels(o.baseName, commonresources.LabelValueK8sComponentGateway)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(o.baseName)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyServiceName,
			Namespace: o.globals.TargetNamespace(),
			Labels:    commonLabels,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPGRPC,
					TargetPort: intstr.FromInt32(ports.OTLPGRPC),
				},
				{
					Name:       "http-collector",
					Protocol:   corev1.ProtocolTCP,
					Port:       ports.OTLPHTTP,
					TargetPort: intstr.FromInt32(ports.OTLPHTTP),
				},
			},
			Selector:              selectorLabels,
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
		},
	}
}

func (o *OTLPGatewayApplierDeleter) makeGatewayDaemonSet(configChecksum string, opts GatewayApplyOptions) *appsv1.DaemonSet {
	podSpec := o.makeGatewayPodSpec(opts)
	metadata := o.makeGatewayMetadata(configChecksum, opts)

	return makeGatewayDaemonSet(
		o.baseName,
		o.globals.TargetNamespace(),
		metadata,
		podSpec,
	)
}

// makeGatewayPodSpec creates the pod spec for gateway (Deployment or DaemonSet)
//
//nolint:dupl // repeating the code as we this would be deleted when we implement all signals in OTLP gateway
func (o *OTLPGatewayApplierDeleter) makeGatewayPodSpec(opts GatewayApplyOptions) corev1.PodSpec {
	resources := o.makeGatewayResourceRequirements(opts)

	containerOpts := slices.Clone(o.containerOpts)
	containerOpts = append(containerOpts,
		commonresources.WithResources(resources),
		commonresources.WithGoMemLimitEnvVar(resources.Limits[corev1.ResourceMemory]),
		commonresources.WithClusterTrustBundleVolumeMount(o.globals.ClusterTrustBundleName()),
	)

	podOptions := make([]commonresources.PodSpecOption, 0)
	podOptions = append(podOptions, o.podOpts...)
	podOptions = append(podOptions, commonresources.WithImagePullSecretName(o.globals.ImagePullSecretName()),
		commonresources.WithClusterTrustBundleVolume(o.globals.ClusterTrustBundleName()),
	)

	return makePodSpec(
		o.baseName,
		o.image,
		podOptions,
		containerOpts,
	)
}

func (o *OTLPGatewayApplierDeleter) makeGatewayResourceRequirements(opts GatewayApplyOptions) corev1.ResourceRequirements {
	memoryRequest := o.baseMemoryRequest.DeepCopy()
	memoryLimit := o.baseMemoryLimit.DeepCopy()
	cpuRequest := o.baseCPURequest.DeepCopy()

	for range opts.ResourceRequirementsMultiplier {
		memoryRequest.Add(o.dynamicMemoryRequest)
		memoryLimit.Add(o.dynamicMemoryLimit)
		cpuRequest.Add(o.dynamicCPURequest)
	}

	resources := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceMemory: memoryLimit,
		},
	}

	return resources
}

// makeGatewayMetadata prepares labels and annotations for gateway resources
func (o *OTLPGatewayApplierDeleter) makeGatewayMetadata(configChecksum string, opts GatewayApplyOptions) WorkloadMetadata {
	annotations := o.makeAnnotations(configChecksum, opts)

	return MakeWorkloadMetadata(
		&o.globals,
		o.baseName,
		commonresources.LabelValueK8sComponentGateway,
		o.extraPodLabels,
		annotations,
	)
}

func (o *OTLPGatewayApplierDeleter) makeAnnotations(configChecksum string, opts GatewayApplyOptions) map[string]string {
	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}

	if opts.IstioEnabled {
		// exclude all inbound ports from service mesh
		annotations[commonresources.AnnotationKeyIstioIncludeInboundPorts] = ""
		// When a workload is outside the istio mesh and communicates with pod in service mesh, the envoy proxy does not
		// preserve the source IP and destination IP. To preserve source/destination IP we need TPROXY interception mode.
		// More info: https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-InboundInterceptionMode
		annotations[commonresources.AnnotationKeyIstioInterceptionMode] = commonresources.AnnotationValueIstioInterceptionModeTProxy
	}

	return annotations
}
