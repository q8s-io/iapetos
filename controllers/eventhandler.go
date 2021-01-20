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

const StatefulPod = "StatefulPod"

type StatefulPodEvent struct{}

func (s StatefulPodEvent) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	if event.Meta == nil {
		log.Error(nil, "CreateEvent received with no metadata", "event", event)
		return
	}
	if pod, ok := event.Object.(*corev1.Pod); ok {
		if _, ok := pod.Annotations[statefulpodv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
}

func (s StatefulPodEvent) Update(event event.UpdateEvent, q workqueue.RateLimitingInterface) {
	if event.MetaNew == nil {
		log.Error(nil, "CreateEvent received with no metadata", "event", event)
		return
	}
	if pod, ok := event.ObjectNew.(*corev1.Pod); ok {
		if _, ok := pod.Annotations[statefulpodv1.GroupVersion.String()]; ok {
			/*fmt.Println(pod.Name+"-------------"+string(pod.Status.Phase))*/
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
}

func (s StatefulPodEvent) Delete(event event.DeleteEvent, q workqueue.RateLimitingInterface) {
	if event.Meta == nil {
		log.Error(nil, "CreateEvent received with no metadata", "event", event)
		return
	}
	if pod, ok := event.Object.(*corev1.Pod); ok {
		if _, ok := pod.Annotations[statefulpodv1.GroupVersion.String()]; ok {
			/*fmt.Println(pod.Name+"-------------"+string(pod.Status.Phase))*/
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
}

func (s StatefulPodEvent) Generic(event.GenericEvent, workqueue.RateLimitingInterface) {

}

func isRunning(pod *corev1.Pod) bool {
	if pod.Status.Phase == corev1.PodRunning {
		return true
	}
	return false
}
