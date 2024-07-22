package workloadstatus

import (
	"context"
	"errors"
	"fmt"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var (
	ErrDeploymentNotFound          = errors.New("deployment is not yet created")
	ErrDeploymentFetching          = errors.New("failed to get Deployment")
	ErrFailedToGetLatestReplicaSet = errors.New("failed to get latest ReplicaSets")
)

type FailedToListReplicaSetErr struct {
	Message string
}

func (ftlr *FailedToListReplicaSetErr) Error() string {
	return fmt.Sprintf("failed to list ReplicaSets: %s", ftlr.Message)
}

func IsFailedToListReplicaSetErr(err error) bool {
	var ftlr *FailedToListReplicaSetErr
	return errors.As(err, &ftlr)
}

type FailedToFetchReplicaSetErr struct {
	Message string
}

func (ftfr *FailedToFetchReplicaSetErr) Error() string {
	return fmt.Sprintf("failed to fetch ReplicaSets: %s", ftfr.Message)
}

func IsFailedToFetchReplicaSetErr(err error) bool {
	var ftfr *FailedToFetchReplicaSetErr
	return errors.As(err, &ftfr)
}

type DeploymentProber struct {
	client.Client
}

func (dp *DeploymentProber) IsReady(ctx context.Context, name types.NamespacedName) error {
	log := logf.FromContext(ctx)
	var d appsv1.Deployment
	if err := dp.Get(ctx, name, &d); err != nil {
		if apierrors.IsNotFound(err) {
			// The status of pipeline is changed before the creation of daemonset
			log.V(1).Info("Deployment is not yet created")
			return ErrDeploymentNotFound
		}
		return ErrDeploymentFetching
	}

	desiredReplicas := *d.Spec.Replicas
	var allReplicaSets appsv1.ReplicaSetList

	listOps := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(d.Spec.Selector.MatchLabels),
		Namespace:     d.Namespace,
	}
	if err := dp.List(ctx, &allReplicaSets, listOps); err != nil {
		return &FailedToListReplicaSetErr{Message: err.Error()}
	}

	if err := dp.Get(ctx, name, &d); err != nil {
		return &FailedToFetchReplicaSetErr{Message: err.Error()}
	}

	replicaSet := getLatestReplicaSet(&d, &allReplicaSets)
	if replicaSet == nil {
		return ErrFailedToGetLatestReplicaSet
	}

	if replicaSet.Status.ReadyReplicas >= desiredReplicas {
		return nil
	}
	if err := checkPodStatus(ctx, dp.Client, name.Namespace, d.Spec.Selector); err != nil {
		return err
	}
	return ErrReplicaCountMismatch
}

func getLatestReplicaSet(deployment *appsv1.Deployment, allReplicaSets *appsv1.ReplicaSetList) *appsv1.ReplicaSet {
	var ownedReplicaSets []*appsv1.ReplicaSet
	for i := range allReplicaSets.Items {
		if metav1.IsControlledBy(&allReplicaSets.Items[i], deployment) {
			ownedReplicaSets = append(ownedReplicaSets, &allReplicaSets.Items[i])
		}
	}

	if len(ownedReplicaSets) == 0 {
		return nil
	}

	return findNewReplicaSet(deployment, ownedReplicaSets)
}

// findNewReplicaSet returns the new RS this given deployment targets (the one with the same pod template).
func findNewReplicaSet(deployment *appsv1.Deployment, rsList []*appsv1.ReplicaSet) *appsv1.ReplicaSet {
	sort.Sort(replicaSetsByCreationTimestamp(rsList))
	for i := range rsList {
		if equalIgnoreHash(&rsList[i].Spec.Template, &deployment.Spec.Template) {
			// In rare cases, such as after cluster upgrades, Deployment may end up with
			// having more than one new ReplicaSets that have the same template as its template,
			// see https://github.com/kubernetes/kubernetes/issues/40415
			// We deterministically choose the oldest new ReplicaSet.
			return rsList[i]
		}
	}
	// new ReplicaSet does not exist.
	return nil
}

func equalIgnoreHash(template1, template2 *corev1.PodTemplateSpec) bool {
	t1Copy := template1.DeepCopy()
	t2Copy := template2.DeepCopy()
	delete(t1Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	delete(t2Copy.Labels, appsv1.DefaultDeploymentUniqueLabelKey)
	return apiequality.Semantic.DeepEqual(t1Copy, t2Copy)
}

type replicaSetsByCreationTimestamp []*appsv1.ReplicaSet

func (o replicaSetsByCreationTimestamp) Len() int      { return len(o) }
func (o replicaSetsByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o replicaSetsByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}
