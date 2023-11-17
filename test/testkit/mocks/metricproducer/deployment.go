package metricproducer

//
//import (
//	appsv1 "k8s.io/api/apps/v1"
//	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
//	"maps"
//)
//
//type Deployment struct {
//	name        string
//	namespace   string
//	labels      map[string]string
//	annotations map[string]string
//	pod         *Pod
//}
//
//func (mp *MetricProducer) Deployment() *Deployment {
//	return &Deployment{
//		name:        mp.name,
//		namespace:   mp.namespace,
//		labels:      make(map[string]string),
//		annotations: make(map[string]string),
//		pod:         mp.Pod(),
//	}
//}
//
//func (d *Deployment) WithLabel(key, value string) *Deployment {
//	d.labels[key] = value
//	d.pod = d.pod.WithLabel(key, value)
//	return d
//}
//
//func (d *Deployment) K8sObject() *appsv1.Deployment {
//	labels := d.labels
//	maps.Copy(labels, selectorLabels)
//
//	return &appsv1.Deployment{
//		ObjectMeta: metav1.ObjectMeta{},
//		Spec:       appsv1.DeploymentSpec{},
//	}
//
//}
