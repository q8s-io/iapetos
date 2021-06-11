/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/source"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	statefulpodctrl "github.com/q8s-io/iapetos/controllers/statefulpod"
)

const (
	Pod = "Pod"
	PVC= "PVC"
	StatefulPod="StatefulPod"
	UNKNOWN="UNKNOWN"
)

// StatefulPodReconciler reconciles a StatefulPod object
type StatefulPodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	sync.RWMutex
	watchs    chan struct{}
	deleteEnd chan struct{}
}

// +kubebuilder:rbac:groups=bdg.iapetos.foundary-cloud.io,resources=statefulpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bdg.iapetos.foundary-cloud.io,resources=statefulpods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolume,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolume/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get
func (r *StatefulPodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	switch obj,kind:=r.getType(ctx,req);kind {
	case Pod:
		pod:=obj.(*corev1.Pod)
		return statefulpodctrl.NewStatefulPodCtrl(r.Client).MonitorPodStatus(ctx, pod)
	case PVC:
		pvc:=obj.(*corev1.PersistentVolumeClaim)
		return statefulpodctrl.NewStatefulPodCtrl(r.Client).MonitorPVCStatus(ctx, pvc)
	case StatefulPod:
		statefulPod:=obj.(*iapetosapiv1.StatefulPod)
		return statefulpodctrl.NewStatefulPodCtrl(r.Client).CoreCtrl(ctx, statefulPod)
	}
	return ctrl.Result{}, nil
}

func (r *StatefulPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&iapetosapiv1.StatefulPod{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &StatefulPodEvent{}).
		Watches(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &StatefulPodEvent{}).
		WithEventFilter(StatefulPodPredicate{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 3,
		}).
		Complete(r)
}

func (r *StatefulPodReconciler)getType(ctx context.Context,req ctrl.Request)(interface{},string){
	var pod corev1.Pod
	var pvc corev1.PersistentVolumeClaim
	var statefulPod iapetosapiv1.StatefulPod
	if err:=r.Get(ctx,req.NamespacedName,&statefulPod);err==nil{
		return &statefulPod,StatefulPod
	}
	if err:=r.Get(ctx,req.NamespacedName,&pod);err==nil{
		return &pod,Pod
	}
	if err:=r.Get(ctx,req.NamespacedName,&pvc);err==nil{
		return &pvc,PVC
	}
	return nil,UNKNOWN
}