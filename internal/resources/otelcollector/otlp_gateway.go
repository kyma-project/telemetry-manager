package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"istio.io/api/networking/v1alpha3"
	istiosecurityv1 "istio.io/api/security/v1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istionetworkingclientv1 "istio.io/client-go/pkg/apis/networking/v1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	autoscalingvpav1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/kyma-project/telemetry-manager/internal/config"
	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/k8sclients"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config/common"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/ports"
	commonresources "github.com/kyma-project/telemetry-manager/internal/resources/common"
	"github.com/kyma-project/telemetry-manager/internal/resources/names"
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

var (
	otlpGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	otlpGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	otlpGatewayBaseCPURequest       = resource.MustParse("100m")
	otlpGatewayDynamicCPURequest    = resource.MustParse("100m")
	otlpGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	otlpGatewayDynamicMemoryRequest = resource.MustParse("0")
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
		commonresources.LabelKeyTelemetryTraceIngest:  commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryTraceExport:  commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryLogIngest:    commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryLogExport:    commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryMetricIngest: commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryMetricExport: commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:           commonresources.LabelValueTrue, // inject istio sidecar
	}

	return &OTLPGatewayApplierDeleter{
		globals:              globals,
		baseName:             names.OTLPGateway,
		extraPodLabels:       extraLabels,
		image:                image,
		otlpServiceName:      names.OTLPService,
		rbac:                 makeOTLPGatewayRBAC(globals.TargetNamespace()),
		baseMemoryLimit:      otlpGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   otlpGatewayDynamicMemoryLimit,
		baseCPURequest:       otlpGatewayBaseCPURequest,
		dynamicCPURequest:    otlpGatewayDynamicCPURequest,
		baseMemoryRequest:    otlpGatewayBaseMemoryRequest,
		dynamicMemoryRequest: otlpGatewayDynamicMemoryRequest,
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithAffinity(makePodAffinity(commonresources.DefaultSelector(names.OTLPGateway))),
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
	var (
		name = types.NamespacedName{Namespace: o.globals.TargetNamespace(), Name: o.baseName}
	)

	labelerClient := k8sclients.NewLabeler(c, commonresources.DefaultLabels(o.baseName, commonresources.LabelValueK8sComponentGateway))

	if err := applyCommonResources(ctx, labelerClient, name, o.rbac); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secret := makeSecret(name, opts.CollectorEnvVars)
	if err := k8sutils.CreateOrUpdateSecret(ctx, labelerClient, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
	if err := k8sutils.CreateOrUpdateConfigMap(ctx, labelerClient, configMap); err != nil {
		return fmt.Errorf("failed to create configmap: %w", err)
	}

	configChecksum := configchecksum.Calculate([]corev1.ConfigMap{*configMap}, []corev1.Secret{*secret})

	networkPolicies := makeGatewayNetworkPolicies(name, opts.IstioEnabled)

	for _, np := range networkPolicies {
		if err := k8sutils.CreateOrUpdateNetworkPolicy(ctx, labelerClient, np); err != nil {
			return fmt.Errorf("failed to create agent network policies: %w", err)
		}
	}

	if err := k8sutils.CreateOrUpdateDaemonSet(ctx, labelerClient, o.makeGatewayDaemonSet(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create daemonset: %w", err)
	}

	if err := o.applyVPA(ctx, c, labelerClient, name, opts); err != nil {
		return err
	}

	if err := k8sutils.CreateOrUpdateService(ctx, labelerClient, o.makeOTLPService()); err != nil {
		return fmt.Errorf("failed to create otlp service: %w", err)
	}

	// Create the legacy services for backward compatibility
	// These services use the old names but point to the new DaemonSet
	legacyLogService := o.makeLegacyOTLPService(names.OTLPLogsService)
	if err := k8sutils.CreateOrUpdateService(ctx, labelerClient, legacyLogService); err != nil {
		return fmt.Errorf("failed to create legacy log otlp service: %w", err)
	}

	legacyTraceService := o.makeLegacyOTLPService(names.OTLPTracesService)
	if err := k8sutils.CreateOrUpdateService(ctx, labelerClient, legacyTraceService); err != nil {
		return fmt.Errorf("failed to create legacy trace otlp service: %w", err)
	}

	legacyMetricService := o.makeLegacyOTLPService(names.OTLPMetricsService)
	if err := k8sutils.CreateOrUpdateService(ctx, labelerClient, legacyMetricService); err != nil {
		return fmt.Errorf("failed to create legacy metric otlp service: %w", err)
	}

	if opts.IstioEnabled {
		for _, svcName := range []string{names.OTLPLogsService, names.OTLPTracesService, names.OTLPMetricsService, names.OTLPService} {
			if err := k8sutils.CreateOrUpdateDestinationRule(ctx, labelerClient, o.makeDestinationRule(svcName)); err != nil {
				return fmt.Errorf("failed to create destinationrule: %w", err)
			}
		}

		if err := k8sutils.CreateOrUpdatePeerAuthentication(ctx, labelerClient, o.makePeerAuthentication()); err != nil {
			return fmt.Errorf("failed to create peerauthentication: %w", err)
		}
	}

	return nil
}

// DeleteResources removes all OTLP gateway resources.
func (o *OTLPGatewayApplierDeleter) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool, vpaCRDExists bool) error {
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

	networkPolicySelector := map[string]string{
		commonresources.LabelKeyK8sName: name.Name,
	}

	if err := k8sutils.DeleteObjectsByLabelSelector(ctx, c, &networkingv1.NetworkPolicyList{}, networkPolicySelector); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete network policy: %w", err))
	}

	daemonSet := appsv1.DaemonSet{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &daemonSet); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete daemonset: %w", err))
	}

	if vpaCRDExists {
		vpa := autoscalingvpav1.VerticalPodAutoscaler{ObjectMeta: objectMeta}
		if err := k8sutils.DeleteObject(ctx, c, &vpa); err != nil {
			allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete VPA: %w", err))
		}
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

	legacyTraceService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.OTLPTracesService, Namespace: o.globals.TargetNamespace()}}
	if err := k8sutils.DeleteObject(ctx, c, &legacyTraceService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy trace otlp service: %w", err))
	}

	legacyMetricService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: names.OTLPMetricsService, Namespace: o.globals.TargetNamespace()}}
	if err := k8sutils.DeleteObject(ctx, c, &legacyMetricService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete legacy metric otlp service: %w", err))
	}

	if isIstioActive {
		for _, svcName := range []string{names.OTLPLogsService, names.OTLPTracesService, names.OTLPMetricsService, names.OTLPService} {
			destinationRuleMeta := metav1.ObjectMeta{Namespace: o.globals.TargetNamespace(), Name: svcName}

			destinationRule := istionetworkingclientv1.DestinationRule{ObjectMeta: destinationRuleMeta}
			if err := k8sutils.DeleteObject(ctx, c, &destinationRule); err != nil {
				allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete destinationrule: %w", err))
			}
		}

		peerAuth := istiosecurityclientv1.PeerAuthentication{ObjectMeta: metav1.ObjectMeta{Name: o.baseName, Namespace: o.globals.TargetNamespace()}}
		if err := k8sutils.DeleteObject(ctx, c, &peerAuth); err != nil {
			allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete peerauthentication: %w", err))
		}
	}

	return allErrors
}

// applyVPA creates, updates, or deletes the VPA resource based on configuration.
func (o *OTLPGatewayApplierDeleter) applyVPA(ctx context.Context, c client.Client, labelerClient client.Client, name types.NamespacedName, opts GatewayApplyOptions) error {
	if !opts.VpaCRDExists {
		return nil
	}

	if opts.VpaEnabled {
		vpa := makeVPA(name, o.baseMemoryRequest, opts.VPAMaxAllowedMemory)
		if err := k8sutils.CreateOrUpdateVPA(ctx, labelerClient, vpa); err != nil {
			return fmt.Errorf("failed to create VPA: %w", err)
		}

		return nil
	}

	// If VPA is disabled, ensure that any existing VPA is cleaned up
	vpa := &autoscalingvpav1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
	}
	if err := k8sutils.DeleteObject(ctx, c, vpa); err != nil {
		return fmt.Errorf("failed to delete VPA: %w", err)
	}

	return nil
}

func (o *OTLPGatewayApplierDeleter) makeDestinationRule(name string) *istionetworkingclientv1.DestinationRule {
	return &istionetworkingclientv1.DestinationRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: o.globals.TargetNamespace(),
		},
		Spec: v1alpha3.DestinationRule{
			Host: fmt.Sprintf("%s.%s.svc.cluster.local", name, o.globals.TargetNamespace()),
			TrafficPolicy: &v1alpha3.TrafficPolicy{
				Tls: &v1alpha3.ClientTLSSettings{Mode: v1alpha3.ClientTLSSettings_DISABLE},
			},
		},
	}
}

func (o *OTLPGatewayApplierDeleter) makePeerAuthentication() *istiosecurityclientv1.PeerAuthentication {
	return &istiosecurityclientv1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.baseName,
			Namespace: o.globals.TargetNamespace(),
		},
		Spec: istiosecurityv1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: commonresources.DefaultSelector(o.baseName)},
			Mtls:     &istiosecurityv1.PeerAuthentication_MutualTLS{Mode: istiosecurityv1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
}

func (o *OTLPGatewayApplierDeleter) makeOTLPService() *corev1.Service {
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      o.otlpServiceName,
			Namespace: o.globals.TargetNamespace(),
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
			Selector:              commonresources.DefaultSelector(o.baseName),
			Type:                  corev1.ServiceTypeClusterIP,
			InternalTrafficPolicy: ptr.To(corev1.ServiceInternalTrafficPolicyLocal),
		},
	}

	return service
}

// makeLegacyOTLPService creates a service with a legacy name that points to the unified OTLP gateway
func (o *OTLPGatewayApplierDeleter) makeLegacyOTLPService(legacyServiceName string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyServiceName,
			Namespace: o.globals.TargetNamespace(),
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
			Selector:              commonresources.DefaultSelector(o.baseName),
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

	// When VPA is active, override the memory limit to 2x the memory request so the VPA can scale within a tighter range.
	// This replaces the calculated memory limit with a value based on the memory request.
	// For more details, check the ADR: https://github.com/kyma-project/telemetry-manager/blob/main/docs/contributor/arch/032-vertical-pod-autoscaler-VPA-architecture.md
	if opts.VpaCRDExists && opts.VpaEnabled {
		memoryRequest = o.baseMemoryRequest.DeepCopy()
		vpaMemoryLimit := o.baseMemoryRequest.DeepCopy()
		vpaMemoryLimit.Add(memoryRequest)
		memoryLimit = vpaMemoryLimit
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

type GatewayApplyOptions struct {
	CollectorConfigYAML string
	CollectorEnvVars    map[string][]byte
	IstioEnabled        bool
	// Replicas specifies the number of gateway replicas.
	Replicas int32
	// ResourceRequirementsMultiplier is a coefficient affecting the CPU and memory resource limits for each replica.
	// This value is multiplied with a base resource requirement to calculate the actual CPU and memory limits.
	// A value of 1 applies the base limits; values greater than 1 increase those limits proportionally.
	ResourceRequirementsMultiplier int
	VpaCRDExists                   bool
	VpaEnabled                     bool
	VPAMaxAllowedMemory            resource.Quantity
}

func makePodAffinity(labels map[string]string) corev1.Affinity {
	return corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 100, //nolint:mnd // 100% weight
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: commonresources.LabelKeyK8sHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				{
					Weight: 100, //nolint:mnd // 100% weight
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: commonresources.LabelKeyK8sZone,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
			},
		},
	}
}

func makeGatewayNetworkPolicies(name types.NamespacedName, istioEnabled bool) []*networkingv1.NetworkPolicy {
	var (
		otlpPorts    = gatewayIngressOTLPPorts()
		metricsPorts = gatewayIngressMetricsPorts(istioEnabled)
	)

	metricsNetworkPolicy := commonresources.MakeNetworkPolicy(
		name,
		commonresources.DefaultSelector(name.Name),
		commonresources.WithNameSuffix("metrics"),
		commonresources.WithIngressFromPodsInAllNamespaces(
			map[string]string{
				commonresources.LabelKeyTelemetryMetricsScraping: commonresources.LabelValueTelemetryMetricsScraping,
			},
			metricsPorts,
		),
	)

	gatewayNetworkPolicies := commonresources.MakeNetworkPolicy(
		name,
		commonresources.DefaultSelector(name.Name),
		commonresources.WithIngressFromAny(otlpPorts...),
		commonresources.WithEgressToAny(),
	)

	return []*networkingv1.NetworkPolicy{metricsNetworkPolicy, gatewayNetworkPolicies}
}

func gatewayIngressOTLPPorts() []int32 {
	return []int32{
		ports.OTLPHTTP,
		ports.OTLPGRPC,
	}
}

func gatewayIngressMetricsPorts(istioEnabled bool) []int32 {
	metricsPorts := []int32{ports.Metrics}
	if istioEnabled {
		metricsPorts = append(metricsPorts, ports.IstioEnvoyTelemetry)
	}

	return metricsPorts
}
