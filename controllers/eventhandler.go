package controllers

import (
	"github.com/prometheus/common/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
)

type StatefulPodEvent struct{}

func (s StatefulPodEvent) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {
	if event.Meta == nil {
		log.Error(nil, "CreateEvent received with no metadata", "event", event)
		return
	}
	if pod, ok := event.Object.(*corev1.Pod); ok {
		if _, ok := pod.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
	if pvc, ok := event.Object.(*corev1.PersistentVolumeClaim); ok {
		if _, ok := pvc.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
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
		if _, ok := pod.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
	if pvc, ok := event.ObjectNew.(*corev1.PersistentVolumeClaim); ok {
		if _, ok := pvc.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
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
		if _, ok := pod.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      pod.Name,
			}})
			return
		}
	}
	if pvc, ok := event.Object.(*corev1.PersistentVolumeClaim); ok {
		if _, ok := pvc.Annotations[iapetosapiv1.GroupVersion.String()]; ok {
			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: pvc.Namespace,
				Name:      pvc.Name,
			}})
			return
		}
	}
}

func (s StatefulPodEvent) Generic(event event.GenericEvent, q workqueue.RateLimitingInterface) {
}
