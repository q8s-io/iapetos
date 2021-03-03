package controllers

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	podservice "github.com/q8s-io/iapetos/services/pod"
)

type StatefulPodPredicate struct {
}

func (s StatefulPodPredicate) Create(e event.CreateEvent) bool {
	return true
}

func (s StatefulPodPredicate) Delete(e event.DeleteEvent) bool {
	return true
}

func (s StatefulPodPredicate) Update(e event.UpdateEvent) bool {
	if oldObj, ok := e.ObjectOld.(*iapetosapiv1.StatefulPod); ok {
		newObj, _ := e.ObjectNew.(*iapetosapiv1.StatefulPod)
		if !reflect.DeepEqual(oldObj.Spec.PodTemplate, newObj.Spec.PodTemplate) {
			return false
		}
		if len(newObj.Status.PodStatusMes) != 0 && newObj.Status.PodStatusMes[len(newObj.Status.PodStatusMes)-1].Status == podservice.Preparing {
			return false
		}
		if len(newObj.Status.PVCStatusMes) != 0 && newObj.Status.PVCStatusMes[len(newObj.Status.PVCStatusMes)-1].Status == corev1.ClaimPending {
			return false
		}
		if !reflect.DeepEqual(oldObj.Finalizers, newObj.Finalizers) {
			return false
		}
		if !reflect.DeepEqual(oldObj.Status.PVCStatusMes, newObj.Status.PVCStatusMes) {
			return false
		}
	}
	return true
}

func (s StatefulPodPredicate) Generic(e event.GenericEvent) bool {
	return true
}
