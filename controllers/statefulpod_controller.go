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
	"sort"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.InNamespace(req.Namespace), client.MatchingLabels{statefulpodv1.Label: statefulPod.Name}); err != nil {
		log.Error(err, "unable to list child pods")
		return ctrl.Result{}, err
	}
	sortPort(pods.Items)
	err,exit:=r.ContrlPod(ctx,&statefulPod,pods.Items)
	if err!=nil{
		return ctrl.Result{}, err
	}else if exit{
		return ctrl.Result{},nil
	}
	err = r.CreateByReplicas(ctx, &statefulPod, pods.Items)
	if err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *StatefulPodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&statefulpodv1.StatefulPod{}).
		Owns(&corev1.Pod{}).WithEventFilter(StatefulPodPredicate{}).
		Complete(r)
}

//维护 statefulpod 下的pod
func (r *StatefulPodReconciler) ContrlPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pods []corev1.Pod)(error,bool) {
	for i, v := range pods {
		index, _ := strconv.Atoi(v.ObjectMeta.Annotations["index"])
		if index != i {
			name := statefulPod.ObjectMeta.Name + strconv.Itoa(i)
			pod := statefulPod.CreatePod(name)
			pod.ObjectMeta.Annotations = map[string]string{
				"index": strconv.Itoa(i),
			}
			if err := r.Create(ctx, pod); err != nil {
				return err,true
			}
			return nil,true
		}
	}
	return nil,false
}

//根据replicas 创建pod
func (r *StatefulPodReconciler) CreateByReplicas(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pods []corev1.Pod) error {
	if len(pods) < int(*statefulPod.Spec.Size) {
		for i := len(pods); i < int(*statefulPod.Spec.Size); i++ {
			name := statefulPod.ObjectMeta.Name + strconv.Itoa(i)
			pod := statefulPod.CreatePod(name)
			pod.ObjectMeta.Annotations = map[string]string{
				"index": strconv.Itoa(i),
			}
			if err := r.Create(ctx, pod); err != nil {
				return err
			}
		}
	} else if len(pods) > int(*statefulPod.Spec.Size) {
		for i := len(pods); i > int(*statefulPod.Spec.Size); i-- {
			if err := r.Delete(ctx, &(pods[i-1])); err != nil {
				return err
			}
		}
	}
	return nil
}

func sortPort(pods []corev1.Pod) {
	sort.Slice(pods, func(i, j int) bool {
		v1, _ := strconv.Atoi(pods[i].Annotations["index"])
		v2, _ := strconv.Atoi(pods[j].Annotations["index"])
		return v1 < v2
	})
}
