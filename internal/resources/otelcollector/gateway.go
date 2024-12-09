package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

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

	"github.com/kyma-project/telemetry-manager/internal/configchecksum"
	"github.com/kyma-project/telemetry-manager/internal/otelcollector/config"
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

	// label keys
	logGatewayIngestKey    = "telemetry.kyma-project.io/log-ingest"
	logGatewayExportKey    = "telemetry.kyma-project.io/log-export"
	traceGatewayIngestKey  = "telemetry.kyma-project.io/trace-ingest"
	traceGatewayExportKey  = "telemetry.kyma-project.io/trace-export"
	metricGatewayIngestKey = "telemetry.kyma-project.io/metric-ingest"
	metricGatewayExportKey = "telemetry.kyma-project.io/metric-export"
	istioSidecarInjectKey  = "sidecar.istio.io/inject"
)

var (
	// TODO(skhalash): the resource requirements are copy-pasted from the trace gateway and need to be adjusted
	logGatewayBaseCPULimit         = resource.MustParse("700m")
	logGatewayDynamicCPULimit      = resource.MustParse("500m")
	logGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	logGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	logGatewayBaseCPURequest       = resource.MustParse("100m")
	logGatewayDynamicCPURequest    = resource.MustParse("100m")
	logGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	logGatewayDynamicMemoryRequest = resource.MustParse("0")

	metricGatewayBaseCPULimit         = resource.MustParse("900m")
	metricGatewayDynamicCPULimit      = resource.MustParse("100m")
	metricGatewayBaseMemoryLimit      = resource.MustParse("512Mi")
	metricGatewayDynamicMemoryLimit   = resource.MustParse("512Mi")
	metricGatewayBaseCPURequest       = resource.MustParse("25m")
	metricGatewayDynamicCPURequest    = resource.MustParse("0")
	metricGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	metricGatewayDynamicMemoryRequest = resource.MustParse("0")

	traceGatewayBaseCPULimit         = resource.MustParse("700m")
	traceGatewayDynamicCPULimit      = resource.MustParse("500m")
	traceGatewayBaseMemoryLimit      = resource.MustParse("500Mi")
	traceGatewayDynamicMemoryLimit   = resource.MustParse("1500Mi")
	traceGatewayBaseCPURequest       = resource.MustParse("100m")
	traceGatewayDynamicCPURequest    = resource.MustParse("100m")
	traceGatewayBaseMemoryRequest    = resource.MustParse("32Mi")
	traceGatewayDynamicMemoryRequest = resource.MustParse("0")
)

func NewLogGatewayApplierDeleter(image, namespace, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		logGatewayIngestKey: "true",
		logGatewayExportKey: "true",
	}

	return &GatewayApplierDeleter{
		baseName:             LogGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		namespace:            namespace,
		otlpServiceName:      LogOTLPServiceName,
		priorityClassName:    priorityClassName,
		rbac:                 makeLogGatewayRBAC(namespace),
		baseCPULimit:         logGatewayBaseCPULimit,
		dynamicCPULimit:      logGatewayDynamicCPULimit,
		baseMemoryLimit:      logGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   logGatewayDynamicMemoryLimit,
		baseCPURequest:       logGatewayBaseCPURequest,
		dynamicCPURequest:    logGatewayDynamicCPURequest,
		baseMemoryRequest:    logGatewayBaseMemoryRequest,
		dynamicMemoryRequest: logGatewayDynamicMemoryRequest,
	}
}

func NewMetricGatewayApplierDeleter(image, namespace, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		metricGatewayIngestKey: "true",
		metricGatewayExportKey: "true",
		istioSidecarInjectKey:  "true", // inject istio sidecar
	}

	return &GatewayApplierDeleter{
		baseName:             MetricGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		namespace:            namespace,
		otlpServiceName:      MetricOTLPServiceName,
		priorityClassName:    priorityClassName,
		rbac:                 makeMetricGatewayRBAC(namespace),
		baseCPULimit:         metricGatewayBaseCPULimit,
		dynamicCPULimit:      metricGatewayDynamicCPULimit,
		baseMemoryLimit:      metricGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   metricGatewayDynamicMemoryLimit,
		baseCPURequest:       metricGatewayBaseCPURequest,
		dynamicCPURequest:    metricGatewayDynamicCPURequest,
		baseMemoryRequest:    metricGatewayBaseMemoryRequest,
		dynamicMemoryRequest: metricGatewayDynamicMemoryRequest,
	}
}

func NewTraceGatewayApplierDeleter(image, namespace, priorityClassName string) *GatewayApplierDeleter {
	extraLabels := map[string]string{
		traceGatewayIngestKey: "true",
		traceGatewayExportKey: "true",
		istioSidecarInjectKey: "true", // inject istio sidecar
	}

	return &GatewayApplierDeleter{
		baseName:             TraceGatewayName,
		extraPodLabels:       extraLabels,
		image:                image,
		namespace:            namespace,
		otlpServiceName:      TraceOTLPServiceName,
		priorityClassName:    priorityClassName,
		rbac:                 makeTraceGatewayRBAC(namespace),
		baseCPULimit:         traceGatewayBaseCPULimit,
		dynamicCPULimit:      traceGatewayDynamicCPULimit,
		baseMemoryLimit:      traceGatewayBaseMemoryLimit,
		dynamicMemoryLimit:   traceGatewayDynamicMemoryLimit,
		baseCPURequest:       traceGatewayBaseCPURequest,
		dynamicCPURequest:    traceGatewayDynamicCPURequest,
		baseMemoryRequest:    traceGatewayBaseMemoryRequest,
		dynamicMemoryRequest: traceGatewayDynamicMemoryRequest,
	}
}

type GatewayApplierDeleter struct {
	baseName          string
	extraPodLabels    map[string]string
	image             string
	namespace         string
	otlpServiceName   string
	priorityClassName string
	rbac              rbac

	baseCPULimit         resource.Quantity
	dynamicCPULimit      resource.Quantity
	baseMemoryLimit      resource.Quantity
	dynamicMemoryLimit   resource.Quantity
	baseCPURequest       resource.Quantity
	dynamicCPURequest    resource.Quantity
	baseMemoryRequest    resource.Quantity
	dynamicMemoryRequest resource.Quantity
}

type GatewayApplyOptions struct {
	AllowedPorts        []int32
	CollectorConfigYAML string
	CollectorEnvVars    map[string][]byte
	IstioEnabled        bool
	IstioExcludePorts   []int32
	// Replicas specifies the number of gateway replicas.
	Replicas int32
	// ResourceRequirementsMultiplier is a coefficient affecting the CPU and memory resource limits for each replica.
	// This value is multiplied with a base resource requirement to calculate the actual CPU and memory limits.
	// A value of 1 applies the base limits; values greater than 1 increase those limits proportionally.
	ResourceRequirementsMultiplier int
}

func (gad *GatewayApplierDeleter) ApplyResources(ctx context.Context, c client.Client, opts GatewayApplyOptions) error {
	name := types.NamespacedName{Namespace: gad.namespace, Name: gad.baseName}

	if err := applyCommonResources(ctx, c, name, gad.rbac, opts.AllowedPorts); err != nil {
		return fmt.Errorf("failed to create common resource: %w", err)
	}

	secret := makeSecret(name, opts.CollectorEnvVars)
	if err := k8sutils.CreateOrUpdateSecret(ctx, c, secret); err != nil {
		return fmt.Errorf("failed to create env secret: %w", err)
	}

	configMap := makeConfigMap(name, opts.CollectorConfigYAML)
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

	name := types.NamespacedName{Name: gad.baseName, Namespace: gad.namespace}
	if err := deleteCommonResources(ctx, c, name); err != nil {
		allErrors = errors.Join(allErrors, err)
	}

	objectMeta := metav1.ObjectMeta{
		Name:      gad.baseName,
		Namespace: gad.namespace,
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

	OTLPService := corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: gad.otlpServiceName, Namespace: gad.namespace}}
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
	selectorLabels := commonresources.MakeDefaultLabels(gad.baseName)

	annotations := gad.makeAnnotations(configChecksum, opts)

	resources := gad.makeGatewayResourceRequirements(opts)
	affinity := makePodAffinity(selectorLabels)

	podSpec := makePodSpec(
		gad.baseName,
		gad.image,
		commonresources.WithPriorityClass(gad.priorityClassName),
		commonresources.WithResources(resources),
		withAffinity(affinity),
		withEnvVarFromSource(config.EnvVarCurrentPodIP, fieldPathPodIP),
		withEnvVarFromSource(config.EnvVarCurrentNodeName, fieldPathNodeName),
		commonresources.WithGoMemLimitEnvVar(resources.Limits[corev1.ResourceMemory]),
	)

	podLabels := make(map[string]string)
	maps.Copy(podLabels, selectorLabels)
	maps.Copy(podLabels, gad.extraPodLabels)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gad.baseName,
			Namespace: gad.namespace,
			Labels:    selectorLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.To(opts.Replicas),
			Selector: &metav1.LabelSelector{
				MatchLabels: selectorLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: annotations,
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
	cpuLimit := gad.baseCPULimit.DeepCopy()

	for range opts.ResourceRequirementsMultiplier {
		memoryRequest.Add(gad.dynamicMemoryRequest)
		memoryLimit.Add(gad.dynamicMemoryLimit)
		cpuRequest.Add(gad.dynamicCPURequest)
		cpuLimit.Add(gad.dynamicCPULimit)
	}

	resources := corev1.ResourceRequirements{
		Requests: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuRequest,
			corev1.ResourceMemory: memoryRequest,
		},
		Limits: map[corev1.ResourceName]resource.Quantity{
			corev1.ResourceCPU:    cpuLimit,
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
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: labels,
						},
					},
				},
				{
					Weight: 100, //nolint:mnd // 100% weight
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "topology.kubernetes.io/zone",
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
	labels := commonresources.MakeDefaultLabels(gad.baseName)

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gad.otlpServiceName,
			Namespace: gad.namespace,
			Labels:    labels,
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
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}
}

func (gad *GatewayApplierDeleter) makePeerAuthentication() *istiosecurityclientv1.PeerAuthentication {
	labels := commonresources.MakeDefaultLabels(gad.baseName)

	return &istiosecurityclientv1.PeerAuthentication{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gad.baseName,
			Namespace: gad.namespace,
			Labels:    labels,
		},
		Spec: istiosecurityv1.PeerAuthentication{
			Selector: &istiotypev1beta1.WorkloadSelector{MatchLabels: labels},
			Mtls:     &istiosecurityv1.PeerAuthentication_MutualTLS{Mode: istiosecurityv1.PeerAuthentication_MutualTLS_PERMISSIVE},
		},
	}
}

func (gad *GatewayApplierDeleter) makeAnnotations(configChecksum string, opts GatewayApplyOptions) map[string]string {
	annotations := map[string]string{"checksum/config": configChecksum}

	if opts.IstioEnabled {
		var excludeInboundPorts []string
		for _, p := range opts.IstioExcludePorts {
			excludeInboundPorts = append(excludeInboundPorts, fmt.Sprintf("%d", p))
		}

		annotations["traffic.sidecar.istio.io/excludeInboundPorts"] = strings.Join(excludeInboundPorts, ", ")
		// When a workload is outside the istio mesh and communicates with pod in service mesh, the envoy proxy does not
		// preserve the source IP and destination IP. To preserve source/destination IP we need TPROXY interception mode.
		// More info: https://istio.io/latest/docs/reference/config/istio.mesh.v1alpha1/#ProxyConfig-InboundInterceptionMode
		annotations["sidecar.istio.io/interceptionMode"] = "TPROXY"
	}

	return annotations
}
