package controllers

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	statefulpodv1 "iapetos/api/v1"
	podcontrl "iapetos/controllers/pod"
)

type StatefulPodPredicate struct {
}

func (s StatefulPodPredicate) Create(e event.CreateEvent) bool {
	return true
}

func (s StatefulPodPredicate) Delete(e event.DeleteEvent) bool {
	/*if obj,ok:=e.Object.(*statefulpodv1.StatefulPod);ok{
		fmt.Println("statefulPod deleted----------",obj.DeletionTimestamp)
		return true
	}*/
	return true
}

func (s StatefulPodPredicate) Update(e event.UpdateEvent) bool {
	if oldObj, ok := e.ObjectOld.(*statefulpodv1.StatefulPod); ok {
		newObj, _ := e.ObjectNew.(*statefulpodv1.StatefulPod)
		if !reflect.DeepEqual(oldObj.Spec.PodTemplate, newObj.Spec.PodTemplate) {
			return false
		}
		if len(newObj.Status.PodStatusMes) != 0 && newObj.Status.PodStatusMes[len(newObj.Status.PodStatusMes)-1].Status == podcontrl.Preparing {
			return false
		}
		if len(newObj.Status.PvcStatusMes) != 0 && newObj.Status.PvcStatusMes[len(newObj.Status.PvcStatusMes)-1].Status == corev1.ClaimPending {
			return false
		}
		if !reflect.DeepEqual(oldObj.Finalizers, newObj.Finalizers) {
			return false
		}
		if !reflect.DeepEqual(oldObj.Status.PvcStatusMes, newObj.Status.PvcStatusMes) {
			return false
		}
	}
	return true
}

func (s StatefulPodPredicate) Generic(e event.GenericEvent) bool {
	return true
}
