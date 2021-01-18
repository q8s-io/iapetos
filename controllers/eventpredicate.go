package controllers

import (
	corev1 "k8s.io/api/core/v1"
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
	if _,ok:=e.ObjectOld.(*corev1.Pod);ok{
		return true
	}
	if oldObj, ok := e.ObjectOld.(*statefulpodv1.StatefulPod); ok {
		if newObj, ok := e.ObjectNew.(*statefulpodv1.StatefulPod); ok {
			if newObj.Annotations["index"]=="0"{
				return false
			}
			if *oldObj.Spec.Size != *newObj.Spec.Size || oldObj.Annotations["index"]!=newObj.Annotations["index"]{
				//fmt.Println("index","------------------",oldObj.Annotations["index"])
				return true
			}
		}
	}
	return false
}

func (s StatefulPodPredicate) Generic(e event.GenericEvent) bool {
	return true
}
