package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	istiosecurityv1 "istio.io/api/security/v1"
	istiotypev1beta1 "istio.io/api/type/v1beta1"
	istiosecurityclientv1 "istio.io/client-go/pkg/apis/security/v1"
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
	k8sutils "github.com/kyma-project/telemetry-manager/internal/utils/k8s"
)

const (
	LogGatewayName    = "telemetry-log-gateway"
	MetricGatewayName = "telemetry-metric-gateway"
	TraceGatewayName  = "telemetry-trace-gateway"

	MetricOTLPServiceName = "telemetry-otlp-metrics"
	TraceOTLPServiceName  = "telemetry-otlp-traces"
	LogOTLPServiceName    = "telemetry-otlp-logs"
)

var (
	// TODO(skhalash): the resource requirements are copy-pasted from the trace gateway and need to be adjusted
	logGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	logGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	logGatewayBaseCPURequest       = resource.MustParse("100m")
	logGatewayDynamicCPURequest    = resource.MustParse("100m")
	logGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	logGatewayDynamicMemoryRequest = resource.MustParse("0")

	metricGatewayBaseMemoryLimit      = resource.MustParse("512Mi")
	metricGatewayDynamicMemoryLimit   = resource.MustParse("512Mi")
	metricGatewayBaseCPURequest       = resource.MustParse("25m")
	metricGatewayDynamicCPURequest    = resource.MustParse("0")
	metricGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	metricGatewayDynamicMemoryRequest = resource.MustParse("0")

	traceGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	traceGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	traceGatewayBaseCPURequest       = resource.MustParse("100m")
	traceGatewayDynamicCPURequest    = resource.MustParse("100m")
	traceGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	traceGatewayDynamicMemoryRequest = resource.MustParse("0")
)

type GatewayApplierDeleter struct {
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
}

//nolint:dupl // repeating the code as we have three different signals
func NewLogGatewayApplierDeleter(globals config.Global, image, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryLogIngest: commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryLogExport: commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:        commonresources.LabelValueTrue, // inject istio sidecar
	}

	return &GatewayApplierDeleter{
		globals:              globals,
		baseName:             LogGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		otlpServiceName:      LogOTLPServiceName,
		rbac:                 makeLogGatewayRBAC(globals.TargetNamespace()),
		baseMemoryLimit:      logGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   logGatewayDynamicMemoryLimit,
		baseCPURequest:       logGatewayBaseCPURequest,
		dynamicCPURequest:    logGatewayDynamicCPURequest,
		baseMemoryRequest:    logGatewayBaseMemoryRequest,
		dynamicMemoryRequest: logGatewayDynamicMemoryRequest,
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithAffinity(makePodAffinity(commonresources.MakeDefaultSelectorLabels(LogGatewayName))),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
		},
	}
}

//nolint:dupl // repeating the code as we have three different signals
func NewMetricGatewayApplierDeleter(globals config.Global, image, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryMetricIngest: commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryMetricExport: commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:           commonresources.LabelValueTrue, // inject istio sidecar
	}

	return &GatewayApplierDeleter{
		globals:              globals,
		baseName:             MetricGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		otlpServiceName:      MetricOTLPServiceName,
		rbac:                 makeMetricGatewayRBAC(globals.TargetNamespace()),
		baseMemoryLimit:      metricGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   metricGatewayDynamicMemoryLimit,
		baseCPURequest:       metricGatewayBaseCPURequest,
		dynamicCPURequest:    metricGatewayDynamicCPURequest,
		baseMemoryRequest:    metricGatewayBaseMemoryRequest,
		dynamicMemoryRequest: metricGatewayDynamicMemoryRequest,
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithAffinity(makePodAffinity(commonresources.MakeDefaultSelectorLabels(MetricGatewayName))),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
		},
	}
}

//nolint:dupl // repeating the code as we have three different signals
func NewTraceGatewayApplierDeleter(globals config.Global, image, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		commonresources.LabelKeyTelemetryTraceIngest: commonresources.LabelValueTrue,
		commonresources.LabelKeyTelemetryTraceExport: commonresources.LabelValueTrue,
		commonresources.LabelKeyIstioInject:          commonresources.LabelValueTrue, // inject istio sidecar
	}

	return &GatewayApplierDeleter{
		globals:              globals,
		baseName:             TraceGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		otlpServiceName:      TraceOTLPServiceName,
		rbac:                 makeTraceGatewayRBAC(globals.TargetNamespace()),
		baseMemoryLimit:      traceGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   traceGatewayDynamicMemoryLimit,
		baseCPURequest:       traceGatewayBaseCPURequest,
		dynamicCPURequest:    traceGatewayDynamicCPURequest,
		baseMemoryRequest:    traceGatewayBaseMemoryRequest,
		dynamicMemoryRequest: traceGatewayDynamicMemoryRequest,
		podOpts: []commonresources.PodSpecOption{
			commonresources.WithPriorityClass(priorityClassName),
			commonresources.WithAffinity(makePodAffinity(commonresources.MakeDefaultSelectorLabels(TraceGatewayName))),
		},
		containerOpts: []commonresources.ContainerOption{
			commonresources.WithEnvVarFromField(common.EnvVarCurrentPodIP, fieldPathPodIP),
			commonresources.WithEnvVarFromField(common.EnvVarCurrentNodeName, fieldPathNodeName),
			commonresources.WithFIPSGoDebugEnvVar(globals.OperateInFIPSMode()),
		},
	}
}

func (gad *GatewayApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error {
	name := types.NamespacedName{Namespace: gad.globals.TargetNamespace(), Name: gad.baseName}

	ingressAllowedPorts := gatewayIngressAllowedPorts()
	if opts.IstioEnabled {
		ingressAllowedPorts = append(ingressAllowedPorts, ports.IstioEnvoy)
	}

	if err := applyCommonResources(ctx, c, name, commonresources.LabelValueK8sComponentGateway, gad.rbac, ingressAllowedPorts); err != nil {
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
	if err := k8sutils.CreateOrUpdateDeployment(ctx, c, gad.makeGatewayDeployment(configChecksum, opts)); err != nil {
		return fmt.Errorf("failed to create deployment: %w", err)
	}

	if err := k8sutils.CreateOrUpdateService(ctx, c, gad.makeOTLPService()); err != nil {
		return fmt.Errorf("failed to create otlp service: %w", err)
	}

	if opts.IstioEnabled {
		if err := k8sutils.CreateOrUpdatePeerAuthentication(ctx, c, gad.makePeerAuthentication()); err != nil {
			return fmt.Errorf("failed to create peerauthentication: %w", err)
		}
	}

	return nil
}

func (gad *GatewayApplierDeleter) DeleteResources(ctx context.Context, c client.Client, isIstioActive bool) error {
	// Attempt to clean up as many resources as possible and avoid early return when one of the deletions fails
	var allErrors error = nil

	name := types.NamespacedName{Name: gad.baseName, Namespace: gad.globals.TargetNamespace()}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      gad.baseName,
		Namespace: gad.globals.TargetNamespace(),
	}

	secret := corev1.Secret{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &secret); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete env secret: %w", err))
	}

	configMap := corev1.ConfigMap{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &configMap); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete configmap: %w", err))
	}

	deployment := appsv1.Deployment{ObjectMeta: objectMeta}
	if err := k8sutils.DeleteObject(ctx, c, &deployment); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete deployment: %w", err))
	}

	OTLPService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: gad.otlpServiceName, Namespace: gad.globals.TargetNamespace()}}
	if err := k8sutils.DeleteObject(ctx, c, &OTLPService); err != nil {
		allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete otlp service: %w", err))
	}

	if isIstioActive {
		peerAuthentication := istiosecurityclientv1.PeerAuthentication{ObjectMeta: objectMeta}
		if err := k8sutils.DeleteObject(ctx, c, &peerAuthentication); err != nil {
			allErrors = errors.Join(allErrors, fmt.Errorf("failed to delete peerauthentication: %w", err))
		}
	}

	return allErrors
}

func (gad *GatewayApplierDeleter) makeGatewayDeployment(configChecksum string, opts GatewayApplyOptions) *appsv1.Deployment {
	labels := commonresources.MakeDefaultLabels(gad.baseName, commonresources.LabelValueK8sComponentGateway)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(gad.baseName)
	annotations := gad.makeAnnotations(configChecksum, opts)

	// Create final annotations for the DaemonSet and Pods with additional annotations
	podAnnotations := make(map[string]string)
	resourceAnnotations := make(map[string]string)

	maps.Copy(resourceAnnotations, gad.globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, gad.globals.AdditionalAnnotations())
	maps.Copy(podAnnotations, annotations)

	defaultPodLabels := make(map[string]string)
	maps.Copy(defaultPodLabels, labels)
	maps.Copy(defaultPodLabels, gad.extraPodLabels)

	// Create final labels for the DaemonSet and Pods with additional labels
	resourceLabels := make(map[string]string)
	podLabels := make(map[string]string)

	maps.Copy(resourceLabels, gad.globals.AdditionalLabels())
	maps.Copy(podLabels, gad.globals.AdditionalLabels())
	maps.Copy(resourceLabels, labels)
	maps.Copy(podLabels, defaultPodLabels)

	resources := gad.makeGatewayResourceRequirements(opts)

	containerOpts := slices.Clone(gad.containerOpts)
	containerOpts = append(containerOpts,
		commonresources.WithResources(resources),
		commonresources.WithGoMemLimitEnvVar(resources.Limits[corev1.ResourceMemory]),
		commonresources.WithClusterTrustBundleVolumeMount(gad.globals.ClusterTrustBundleName()),
	)

	podOptions := make([]commonresources.PodSpecOption, 0)
	podOptions = append(podOptions, gad.podOpts...)
	podOptions = append(podOptions, commonresources.WithImagePullSecretName(gad.globals.ImagePullSecretName()),
		commonresources.WithClusterTrustBundleVolume(gad.globals.ClusterTrustBundleName()),
	)

	podSpec := makePodSpec(
		gad.baseName,
		gad.image,
		podOptions,
		containerOpts,
	)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        gad.baseName,
			Namespace:   gad.globals.TargetNamespace(),
			Labels:      resourceLabels,
			Annotations: resourceAnnotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}
}

func (gad *GatewayApplierDeleter) makeGatewayResourceRequirements(opts GatewayApplyOptions) corev1.ResourceRequirements {
	memoryRequest := gad.baseMemoryRequest.DeepCopy()
	memoryLimit := gad.baseMemoryLimit.DeepCopy()
	cpuRequest := gad.baseCPURequest.DeepCopy()

	for range opts.ResourceRequirementsMultiplier {
		memoryRequest.Add(gad.dynamicMemoryRequest)
		memoryLimit.Add(gad.dynamicMemoryLimit)
		cpuRequest.Add(gad.dynamicCPURequest)
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

func (gad *GatewayApplierDeleter) makeOTLPService() *corev1.Service {
	commonLabels := commonresources.MakeDefaultLabels(gad.baseName, commonresources.LabelValueK8sComponentGateway)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(gad.baseName)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gad.otlpServiceName,
			Namespace: gad.globals.TargetNamespace(),
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
			Selector: selectorLabels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func (gad *GatewayApplierDeleter) makePeerAuthentication() *istiosecurityclientv1.PeerAuthentication {
	commonLabels := commonresources.MakeDefaultLabels(gad.baseName, commonresources.LabelValueK8sComponentGateway)
	selectorLabels := commonresources.MakeDefaultSelectorLabels(gad.baseName)

	return &istiosecurityclientv1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gad.baseName,
			Namespace: gad.globals.TargetNamespace(),
			Labels:    commonLabels,
		},
		Spec: istiosecurityv1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: selectorLabels},
			Mtls:     &istiosecurityv1.PeerAuthentication_MutualTLS{Mode: istiosecurityv1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
}

func (gad *GatewayApplierDeleter) makeAnnotations(configChecksum string, opts GatewayApplyOptions) map[string]string {
	annotations := map[string]string{commonresources.AnnotationKeyChecksumConfig: configChecksum}

	if opts.IstioEnabled {
		annotations[commonresources.AnnotationKeyIstioExcludeInboundPorts] = fmt.Sprintf("%d", ports.Metrics)
		// When a workload is outside the istio mesh and communicates with pod in service mesh, the envoy proxy does not
		// preserve the source IP and destination IP. To preserve source/destination IP we need TPROXY interception mode.
		// More info: https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-InboundInterceptionMode
		annotations[commonresources.AnnotationKeyIstioInterceptionMode] = commonresources.AnnotationValueIstioInterceptionModeTProxy
	}

	return annotations
}

func gatewayIngressAllowedPorts() []int32 {
	return []int32{
		ports.Metrics,
		ports.HealthCheck,
		ports.OTLPHTTP,
		ports.OTLPGRPC,
	}
}
