package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"

	statefulpodv1 "iapetos/api/v1"
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
	if oldObj, ok := e.ObjectOld.(*statefulpodv1.StatefulPod); ok {
		if newObj, ok := e.ObjectNew.(*statefulpodv1.StatefulPod); ok {
			if *oldObj.Spec.Size != *newObj.Spec.Size {
				return true
			}
		}
	}
	return false
}

func (s StatefulPodPredicate) Generic(e event.GenericEvent) bool {
	return true
}
