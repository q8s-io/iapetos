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
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/source"

	statefulpodv1 "iapetos/api/v1"
	pod2 "iapetos/controllers/pod"
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

func (r *StatefulPodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	//fmt.Println(req.Name)
	ctx := context.Background()
	//log := r.Log.WithValues("statefulpod", req.NamespacedName)
	var statefulPod statefulpodv1.StatefulPod
	var pod corev1.Pod
	// statefulPod
	if err := r.Get(ctx, req.NamespacedName, &statefulPod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	} else {
		if len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) {
			if err := r.expansion(ctx, &statefulPod, len(statefulPod.Status.PodStatusMes)); err != nil {
				return ctrl.Result{}, err
			}
		} else if len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) {
			if err := r.shrink(ctx, &statefulPod, len(statefulPod.Status.PodStatusMes)); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err := r.maintain(ctx, &statefulPod); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// pod
	if err := r.Get(ctx, req.NamespacedName, &pod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
	} else {
		_ = r.podHandle(ctx, &pod)
	}
	return ctrl.Result{}, nil
}

func (r *StatefulPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).For(&statefulpodv1.StatefulPod{}).
		Watches(&source.Kind{Type: &corev1.Pod{}}, &StatefulPodEvent{}).
		WithEventFilter(StatefulPodPredicate{}).
		Complete(r)
}

// 扩容
func (r *StatefulPodReconciler) expansion(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	r.Lock()
	defer r.Unlock()
	name := statefulPod.Name + strconv.Itoa(index)
	if _, err, ok := r.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      name,
	}); !ok && err == nil {
		newPod := statefulPod.CreatePod(name)
		newPod.Annotations["index"] = strconv.Itoa(index)
		if err := r.Create(ctx, newPod); err != nil {
			return err
		}
		podIndex := int32(index)
		statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, statefulpodv1.PodStatus{
			PodName: newPod.Name,
			Status:  pod2.Prepared,
			Index:   &podIndex,
		})
		if err := r.Update(ctx, statefulPod); err != nil {
			return err
		}
	}
	return nil
}

// pod 处理
func (r *StatefulPodReconciler) podHandle(ctx context.Context, pod *corev1.Pod) error {
	var statefulPod statefulpodv1.StatefulPod
	if err := r.Get(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[statefulpodv1.ParentNmae],
	}, &statefulPod); err != nil {
		return err
	}
	index := strToInt(pod.Annotations["index"])
	if !pod.DeletionTimestamp.IsZero() {
		if statefulPod.Status.PodStatusMes[index].Status == pod2.Deleted {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].Status = pod2.Deleted
		if err := r.Update(ctx, &statefulPod); err != nil {
			return err
		}
		return nil
	}
	if pod.Status.Phase == corev1.PodRunning {
		//fmt.Println(pod.Name+"-------------"+string(pod.Status.Phase))
		if statefulPod.Status.PodStatusMes[index].Status == corev1.PodRunning {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[index].NodeName = pod.Spec.NodeName
		if err := r.Update(ctx, &statefulPod); err != nil {
			return err
		}
	}
	if pod.CreationTimestamp.Sub(time.Now()) == time.Minute*2 && pod.Status.Phase != corev1.PodRunning {

	}
	return nil
}

// 缩容
func (r *StatefulPodReconciler) shrink(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	r.Lock()
	defer r.Unlock()
	podName := statefulPod.Status.PodStatusMes[index-1].PodName
	if pod, err, ok := r.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok {
		if !pod.DeletionTimestamp.IsZero() {
			return nil
		}
		if err := r.Delete(ctx, pod); err != nil {
			return err
		}
	} else if err == nil && !ok {
		statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
		if err := r.Update(ctx, statefulPod); err != nil {
			return err
		}
	}
	return nil
}

// 维护pod
func (r *StatefulPodReconciler) maintain(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	for i, pod := range statefulPod.Status.PodStatusMes {
		if pod.Status == pod2.Deleted {
			//	fmt.Println(pod.PodName, "----------------------", "delete")
			if _, err, ok := r.IsPodExist(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); err == nil && !ok {
				pod := statefulPod.CreatePod(pod.PodName)
				pod.Annotations["index"] = strconv.Itoa(i)
				if err := r.Create(ctx, pod); err != nil {
					return err
				} else {
					statefulPod.Status.PodStatusMes[i].Status = pod2.Prepared
					if err := r.Update(ctx, statefulPod); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (r *StatefulPodReconciler) IsPodExist(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Pod, error, bool) {
	var pod corev1.Pod
	if err := r.Get(ctx, namespaceName, &pod); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		return nil, err, false
	} else {
		return &pod, nil, true
	}
}

func strToInt(str string) int {
	v, _ := strconv.Atoi(str)
	return v
}
