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
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
)

// StatefulPodReconciler reconciles a StatefulPod object
type StatefulPodReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
	//Recorder record.EventRecorder
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
	//创建pod
	if len(statefulPod.Status.PodStatusMes)==0{
		err:=r.createPod(ctx,&statefulPod,0,int(*statefulPod.Spec.Size))
		if err!=nil{
			return ctrl.Result{}, err
		}
	}
	if int(*statefulPod.Spec.Size) > len(statefulPod.Status.PodStatusMes){
		err:=r.expansion(ctx,&statefulPod)
		if err!=nil{
			return ctrl.Result{}, err
		}
	}else if int(*statefulPod.Spec.Size) < len(statefulPod.Status.PodStatusMes){
		err:=r.shrink(ctx,&statefulPod)
		if err!=nil{
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func (r *StatefulPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&statefulpodv1.StatefulPod{}).
		Owns(&corev1.Pod{}).WithEventFilter(StatefulPodPredicate{}).
		Complete(r)
}

func (r *StatefulPodReconciler)createPod(ctx context.Context,statefulPod *statefulpodv1.StatefulPod,leftIndex,rightIndex int)error{
	next:=make(chan struct{})
	defer close(next)
	for i:=leftIndex;i<rightIndex;i++{
		name:=statefulPod.Name+strconv.Itoa(i)
		pod:=statefulPod.CreatePod(name)
		pod.Annotations= map[string]string{
			"index":strconv.Itoa(i),
		}
		fmt.Println("-----------",pod.Name)
		if err:=r.Create(ctx,pod);err!=nil{
			fmt.Println("create pod err: ",err.Error())
			return err
		}
		go func() {
			for {
				_=r.Get(ctx,types.NamespacedName{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},pod)
				if statefulPod.IsRunning(pod) {
					next <- struct{}{}
					return
				}
			}
		}()
		<-next
		index:=int32(i)
		statefulPod.Status.PodStatusMes=append(statefulPod.Status.PodStatusMes, statefulpodv1.PodStatus{
			PodName:  pod.Name,
			Status:   corev1.PodRunning,
			Index:    &index,
			NodeName: pod.Spec.NodeName,
		})
		if err:=r.Update(ctx,statefulPod);err!=nil{
			return err
		}
	}
	return nil
}

//扩容
func (r *StatefulPodReconciler)expansion(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error{
	return r.createPod(ctx,statefulPod,len(statefulPod.Status.PodStatusMes),int(*statefulPod.Spec.Size))
}

//缩容
func (r *StatefulPodReconciler)shrink(ctx context.Context,statefulPod *statefulpodv1.StatefulPod)error{
	next:=make(chan struct{})
	defer close(next)
	for i:=len(statefulPod.Status.PodStatusMes);i>int(*statefulPod.Spec.Size);i--{
		var pod corev1.Pod
		if err:=r.Get(ctx,types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      statefulPod.Status.PodStatusMes[i-1].PodName,
		},&pod);err!=nil{
			return err
		}
		if err:=r.Delete(ctx,&pod);err!=nil{
			return err
		}
		go func() {
			for  {
				if err:=r.Get(ctx,types.NamespacedName{
					Namespace: pod.Namespace,
					Name: pod.Name,
				},&pod);err!=nil{
					next<- struct{}{}
					return
				}
			}
		}()
		<-next
		statefulPod.Status.PodStatusMes=statefulPod.Status.PodStatusMes[:len(statefulPod.Status.PodStatusMes)-1]
		if err:=r.Update(ctx,statefulPod);err!=nil{
			return err
		}
	}
	return nil
}


