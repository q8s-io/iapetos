package controllers

import (
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

type StatefulPodEvent struct{}

func (s *StatefulPodEvent) Create(event event.CreateEvent, q workqueue.RateLimitingInterface) {

}

func (s *StatefulPodEvent) Update(event.UpdateEvent, workqueue.RateLimitingInterface) {

}

func (s *StatefulPodEvent) Delete(event.DeleteEvent, workqueue.RateLimitingInterface) {

}

func (s *StatefulPodEvent) Generic(event.GenericEvent, workqueue.RateLimitingInterface) {

}
