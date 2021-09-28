package controllers

import (
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"

	iapetosapiv1 "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/api/v1"
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
