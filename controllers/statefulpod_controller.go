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
	"strconv"
	"sync"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	statefulpodv1 "iapetos/api/v1"
)

// StatefulPodReconciler reconciles a StatefulPod object
type StatefulPodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	sync.RWMutex
	watchs chan struct{}
	deleteEnd chan struct{}
}

// +kubebuilder:rbac:groups=bdg.iapetos.foundary-cloud.io,resources=statefulpods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=bdg.iapetos.foundary-cloud.io,resources=statefulpods/status,verbs=get;update;patch

func (r *StatefulPodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("statefulpod", req.NamespacedName)
	var statefulPod statefulpodv1.StatefulPod
	if err := r.Get(ctx, req.NamespacedName, &statefulPod); err != nil {
		log.Error(err, "unable to fetch statefulPod")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if _,ok:=statefulPod.Annotations["index"];!ok{
		statefulPod.Annotations= map[string]string{"index":"0"}
		_=r.Update(ctx,&statefulPod)
	}
	index:=stringToInt(statefulPod.Annotations["index"])
	if err:=r.HandlePod(ctx,&statefulPod,index,int(*statefulPod.Spec.Size));err!=nil{
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *StatefulPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&statefulpodv1.StatefulPod{}).Watches(&source.Kind{Type: &corev1.Pod{}},&StatefulPodEvent{}).
		WithEventFilter(StatefulPodPredicate{}).
		Complete(r)
}

func (r *StatefulPodReconciler)HandlePod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, leftIndex, rightIndex int) error{
	if leftIndex<rightIndex{
		if err:=r.createPod(ctx,statefulPod,leftIndex);err!=nil{
			return err
		}
	}
	return nil
}

// 创建pod
func (r *StatefulPodReconciler) createPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	r.Lock()
	defer r.Unlock()
	var pod corev1.Pod
	name := statefulPod.Name + strconv.Itoa(index)
	if err:=r.Get(ctx,types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      name,
	},&pod);err!=nil{
		newPod:=statefulPod.CreatePod(name)
		if err=r.Create(ctx,newPod);err!=nil{
			return err
		}
		podIndex:=int32(index)
		statefulPod.Status.PodStatusMes=append(statefulPod.Status.PodStatusMes,statefulpodv1.PodStatus{
			PodName:  newPod.Name,
			Index:    &podIndex,
		})
		_=r.Update(ctx,statefulPod)
		return nil
	}
	if index==len(statefulPod.Status.PodStatusMes){  //防止statefulpod PodStatusMes 更新未同步
		return nil
	}
	statefulPod.Status.PodStatusMes[index].Status=pod.Status.Phase
	statefulPod.Status.PodStatusMes[index].NodeName=pod.Spec.NodeName
	_=r.Update(ctx,statefulPod)
	if pod.Status.Phase==corev1.PodRunning{
		statefulPod.Annotations["index"]=strconv.Itoa(index+1)
		_=r.Update(ctx,statefulPod)
	}
	return nil
}

func stringToInt(string2 string)int{
	v,_:=strconv.Atoi(string2)
	return v
}
