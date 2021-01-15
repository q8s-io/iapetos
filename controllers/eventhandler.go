package controllers

import (
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	statefulpodv1 "iapetos/api/v1"
)

const StatefulPod="StatefulPod"

type StatefulPodEvent struct{}

func (s StatefulPodEvent) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	if event.Meta==nil{
		log.Error(nil, "CreateEvent received with no metadata", "event", event)
		return
	}
	if _,ok:=event.Object.(*statefulpodv1.StatefulPod);ok{
		q.Add(reconcile.Request{NamespacedName:types.NamespacedName{
			Namespace: event.Meta.GetName(),
			Name:      event.Meta.GetNamespace(),
		}})
		return
	}
	if pod,ok:=event.Object.(*corev1.Pod);ok && pod.Namespace==event.Meta.GetNamespace(){
		for _,own:=range pod.OwnerReferences{
			if own.Kind==StatefulPod && own.APIVersion==statefulpodv1.GroupVersion.String(){
				if isRunning(pod){
					q.Add(reconcile.Request{NamespacedName:types.NamespacedName{
						Namespace: pod.Namespace,
						Name:      event.Meta.GetName(),
					}})
					return
				}
			}
		}
	}
}

func (s StatefulPodEvent) Update(event.UpdateEvent, workqueue.RateLimitingInterface) {

}

func (s StatefulPodEvent) Delete(event.DeleteEvent, workqueue.RateLimitingInterface) {

}

func (s StatefulPodEvent) Generic(event.GenericEvent, workqueue.RateLimitingInterface) {

}

func  isRunning(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodRunning {
		return true
	}
	return false
}
