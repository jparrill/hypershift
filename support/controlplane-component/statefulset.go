package controlplanecomponent

import (
	"context"
	"fmt"

	hyperv1 "github.com/openshift/hypershift/api/hypershift/v1beta1"
	assets "github.com/openshift/hypershift/control-plane-operator/controllers/hostedcontrolplane/v2/assets"
	"github.com/openshift/hypershift/support/util"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ WorkloadProvider[*appsv1.StatefulSet] = &statefulSetProvider{}

type statefulSetProvider struct {
}

func (s *statefulSetProvider) NewObject() *appsv1.StatefulSet {
	return &appsv1.StatefulSet{}
}

// SetReplicasAndStrategy implements WorkloadProvider.
func (d *statefulSetProvider) SetReplicasAndStrategy(object *appsv1.StatefulSet, replicas int32, isRequestServing bool) {
	object.Spec.Replicas = ptr.To(replicas)
	// TODO: should we set any default strategy for statefulsets?
}

// LoadManifest implements WorkloadProvider.
func (s *statefulSetProvider) LoadManifest(componentName string) (*appsv1.StatefulSet, error) {
	return assets.LoadStatefulSetManifest(componentName)
}

// PodTemplateSpec implements WorkloadProvider.
func (s *statefulSetProvider) PodTemplateSpec(object *appsv1.StatefulSet) *corev1.PodTemplateSpec {
	return &object.Spec.Template
}

func (d *statefulSetProvider) Replicas(object *appsv1.StatefulSet) *int32 {
	return object.Spec.Replicas
}

// IsAvailable implements WorkloadProvider.
func (s *statefulSetProvider) IsAvailable(object *appsv1.StatefulSet) (status metav1.ConditionStatus, reason string, message string) {
	// statefulSet is considered available if at least 1 replica is available.
	if ptr.Deref(object.Spec.Replicas, 0) == 0 || object.Status.AvailableReplicas > 0 {
		status = metav1.ConditionTrue
		reason = hyperv1.AsExpectedReason
		message = fmt.Sprintf("StatefulSet %s is available", object.Name)
	} else {
		status = metav1.ConditionFalse
		reason = hyperv1.WaitingForAvailableReason
		message = fmt.Sprintf("StatefulSet %s is not available: %d/%d replicas ready", object.Name, object.Status.ReadyReplicas, *object.Spec.Replicas)
	}
	return
}

// IsReady implements WorkloadProvider.
func (s *statefulSetProvider) IsReady(sts *appsv1.StatefulSet) (status metav1.ConditionStatus, reason string, message string) {
	if util.IsStatefulSetReady(context.TODO(), sts) {
		status = metav1.ConditionTrue
		reason = hyperv1.AsExpectedReason
		message = fmt.Sprintf("Statefulset %s successfully rolled out", sts.Name)
	} else {
		status = metav1.ConditionFalse
		reason = "WaitingForRolloutComplete"
		message = fmt.Sprintf("Waiting for statefulset %s rollout to finish: %d out of %d new replicas have been updated", sts.Name, sts.Status.UpdatedReplicas, *sts.Spec.Replicas)
	}

	return
}
