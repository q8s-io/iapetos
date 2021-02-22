package pod

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	pvservice "iapetos/services/pv"
	pvcservice "iapetos/services/pvc"
	"iapetos/tools"
)

const (
	Preparing   = corev1.PodPhase("Preparing")
	Deleting    = corev1.PodPhase("Deleting")
	ParentNmae  = "parentName"
	StatefulPod = "StatefulPod"
	Index       = "index"
)

type PodService struct {
	client.Client
	Log logr.Logger
}

type PodServiceIntf interface {
	IsPodExist(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Pod, error, bool)
	PodTempale(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, podName string, index int) *corev1.Pod
	CreatePod(ctx context.Context, pod *corev1.Pod) error
	DeletePod(ctx context.Context, pod *corev1.Pod) error
	DeletePodMandatory(ctx context.Context, pod *corev1.Pod, statefulPod *statefulpodv1.StatefulPod) error
	JudgmentPodDel(pod *corev1.Pod) bool
}

func NewPodService(client client.Client) PodServiceIntf {
	//podLog.WithName("pod message")
	return &PodService{client, ctrl.Log.WithName("controllers").WithName("pod")}
}

// 判断 pod 是否存在
func (p *PodService) IsPodExist(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Pod, error, bool) {
	var pod corev1.Pod
	if err := p.Get(ctx, namespaceName, &pod); err != nil {
		if client.IgnoreNotFound(err) == nil { // 找不到改pod
			return nil, nil, false
		}
		p.Log.Error(err, "get pod error")
		return nil, err, false
	} else {
		return &pod, nil, true
	}
}

// 创建 pod 模板
func (p *PodService) PodTempale(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, podName string, index int) *corev1.Pod {
	pvHandler := pvservice.NewPVService(p.Client)
	if len(statefulPod.Spec.PVNames) != 0 && len(statefulPod.Spec.PVNames) > index {
		if pv, ok := pvHandler.IsPVCanUse(ctx, statefulPod.Spec.PVNames[index]); ok {
			nodeName := pv.Annotations["kubevirt.io/provisionOnNode"]
			statefulPod.Spec.PodTemplate.NodeName = nodeName
		}
	}
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				statefulpodv1.GroupVersion.String(): "true",
				ParentNmae:                          statefulPod.Name,
				Index:                               fmt.Sprintf("%v", index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   statefulpodv1.GroupVersion.Group,
					Version: statefulpodv1.GroupVersion.Version,
					Kind:    StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.PodTemplate.DeepCopy(),
	}
	// TODO 判断 pvc 是否需要创建 只支持挂载一个pvc
	if statefulPod.Spec.PvcTemplate != nil {
		pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = pvcservice.NewPvcService(p.Client).SetPvcName(statefulPod, index)
	}
	// 判断 service 是否需要创建，若需要则将标签自动打上
	if statefulPod.Spec.ServiceTemplate != nil {
		pod.Labels = statefulPod.Spec.ServiceTemplate.Selector
	}
	return &pod
}

// 创建 pod
func (p *PodService) CreatePod(ctx context.Context, pod *corev1.Pod) error {
	if err := p.Create(ctx, pod); err != nil {
		//	podLog.Error(err,"create pod error")
		return err
	}
	return nil
}

// 删除 pod
func (p *PodService) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	if err := p.Delete(ctx, pod); err != nil {
		//	podLog.Error(err,"delete pod error")
		return err
	}
	return nil
}

// 立即删除 pod ,节点失联时调用
// 若绑定的有 pvc ，则将 pvc 也进行删除
func (p *PodService) DeletePodMandatory(ctx context.Context, pod *corev1.Pod, statefulPod *statefulpodv1.StatefulPod) error {
	pvchandler := pvcservice.NewPvcService(p.Client)
	if err := p.Delete(ctx, pod, client.DeleteOption(client.GracePeriodSeconds(0)), client.DeleteOption(client.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
		p.Log.Error(err, "delete pod mandatory error")
		return err
	}
	if statefulPod.Spec.PvcTemplate != nil {
		pvcName := pvchandler.SetPvcName(statefulPod, tools.StrToInt(pod.Annotations["index"]))
		if pvc, err, ok := pvchandler.IsPvcExist(ctx, types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pvcName,
		}); err == nil && ok { // pvc 存在
			return pvchandler.DeletePVC(ctx, pvc)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// 判断pod 是否应该是要删除
func (p *PodService) JudgmentPodDel(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return true
	}
	return false
}

// 删除所有 pod , 删除 statefulPod 时调用
// 删除顺序： 先删除pod ，若pvc存在,且 statefulPod 的 PVRecyclePolicy 为 Retain，将pv设置为 Retain ,删除pvc ,将 pv设置为Available
