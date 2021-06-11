package pod_controller

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pvc_controller"
	resourcecfg "github.com/q8s-io/iapetos/initconfig"
	"github.com/q8s-io/iapetos/services"
	podservice "github.com/q8s-io/iapetos/services/pod"
	pvcservice "github.com/q8s-io/iapetos/services/pvc"
)

type PodCtrl struct {
	client.Client
}

const  (
	Preparing   = corev1.PodPhase("Preparing")
	Deleting    = corev1.PodPhase("Deleting")
)

type PodCtrlFunc interface {
	ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error)
	ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) ( bool)
	DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool
	MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int
	MonitorPodStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod, index *int) bool
	PodIsOk(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int
	IsCreationTimeout(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
	//CodbPodReady(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(error)
}

func NewPodCtrl(client client.Client) PodCtrlFunc {
	return &PodCtrl{client}
}

func (podctrl *PodCtrl) IsCreationTimeout(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool {
	podHandler := podservice.NewPodService(podctrl.Client)
	pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	podName:=podHandler.GetName(statefulPod,index)
	if obj,  ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podName,
	}); ok {
		pod := obj.(*corev1.Pod)
		if pod.Status.Phase != corev1.PodRunning && time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) {
			// 删除pod
			if pod.DeletionTimestamp.IsZero() {
				if err := podHandler.Delete(ctx, pod); err != nil {
					return false
				}
				// 如果pvc存在
				if statefulPod.Spec.PVCTemplate!=nil{ // pvc 存在，删除 pvc
					pvcName:=pvcHandler.GetName(statefulPod,index)
					if pvc,ok:=pvcHandler.IsExists(ctx,types.NamespacedName{
						Namespace: statefulPod.Namespace,
						Name:      *pvcName,
					});ok {
						if err := pvcHandler.Delete(ctx, pvc); err != nil {
							return false
						}
					}
				}
				statefulPod.Status.PodStatusMes=statefulPod.Status.PodStatusMes[:index]
				statefulPod.Status.PVCStatusMes=statefulPod.Status.PVCStatusMes[:index]
			}
		}else {
			return true
		}
	}else {
		return true
	}
	return false
}

// 扩容 pod
func (podctrl *PodCtrl) ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error) {
	podHandler := podservice.NewPodService(podctrl.Client)
	podName := podHandler.GetName(statefulPod,index)
	podIndex := int32(index)
	if _,ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podName,
	}); !ok  { // pod 不存在，创建 pod
		podTemplate := podHandler.CreateTemplate(ctx, statefulPod, *podName, index)
		obj,err := podHandler.Create(ctx, podTemplate)
		// 创建失败
		if err != nil {
			return nil, err
		}
		// 记录pod的status
		podStatus := &iapetosapiv1.PodStatus{
			PodName: obj.(*corev1.Pod).Name,
			Status:  Preparing,
			Index:   &podIndex,
		}
		return podStatus, nil
		// pod 存在，podStatus 不变
	} else  {
		podStatus := statefulPod.Status.PodStatusMes[index]
		return &podStatus, nil
	}
}

// 缩容 pod
func (podctrl *PodCtrl) ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (bool) {
	podHandler := podservice.NewPodService(podctrl.Client)
	podName:=podHandler.GetName(statefulPod,index)
	if pod, ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podName,
	}); ok {
		if err := podHandler.Delete(ctx, pod); err != nil {
			return false
		}
		// pod 删除完毕
	} else {
		return  true
	}
	return  false
}

func (podctrl *PodCtrl) DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool {
	podHandler := podservice.NewPodService(podctrl.Client)
	pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	sum:=0
	for i, v := range statefulPod.Status.PodStatusMes {
		if pod, ok := podHandler.IsExists(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PodName,
		}); ok { // pod 存在，删除 pod
			if err := podHandler.Delete(ctx, pod); err != nil {
				return false
			}
			// pod删除完毕删除pvc
			pvcName := pvcHandler.GetName(statefulPod, i)
			if obj, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      *pvcName,
				// pvc 存在，删除 pvc
			}); ok {
				pvc := obj.(*corev1.PersistentVolumeClaim)
				if err := pvcHandler.Delete(ctx, pvc); err != nil {
					return false
				}
			}
		}else { // pvc已经被删除
			sum++
		}
	}
	if sum==len(statefulPod.Status.PVCStatusMes){
		return true
	}
	return false
}


func (podctrl *PodCtrl) MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int {
	podHandler := podservice.NewPodService(podctrl.Client)
	for i, pod := range statefulPod.Status.PodStatusMes {
		// 如果 statefulPod.status.podstatusmes 的状态为 deleting，pod 不存在，返回 pod 索引
		if pod.Status == Deleting {
			if _,  ok := podHandler.IsExists(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName});!ok {
				return &i
			}
		}
	}
	return nil
}

func (podctrl *PodCtrl) PodIsOk(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int {
	podHandler := podservice.NewPodService(podctrl.Client)
	for i, podMsg := range statefulPod.Status.PodStatusMes {
		if _, ok := podHandler.IsExists(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      podMsg.PodName,
		}); !ok {
			statefulPod.Status.PodStatusMes[i].Status = Deleting
			return &i
		}else {
			if podMsg.Status==corev1.PodRunning && statefulPod.Status.PodStatusMes[i].Status!=corev1.PodRunning{
				statefulPod.Status.PodStatusMes[i].Status=corev1.PodRunning
				return &i
			}
		}
	}
	return nil
}

func (podctrl *PodCtrl) MonitorPodStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod, index *int) bool {
	if *index >= len(statefulPod.Status.PodStatusMes) {
		return false
	}
	podHandler := podservice.NewPodService(podctrl.Client)
	pvcHandler:=pvcservice.NewPVCService(podctrl.Client)
	//pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	resourceHandle:=services.NewResource(podctrl.Client)
	if !pod.DeletionTimestamp.IsZero() {
		// 设置过 deleting 状态则不再进行设置
		if statefulPod.Status.PodStatusMes[*index].Status == Deleting {
			return false
		}
		statefulPod.Status.PodStatusMes[*index].Status = Deleting
		return true
	}

	// node Unhealthy
	if !resourceHandle.IsNodeReady(ctx, types.NamespacedName{
		Namespace: "",
		Name:      pod.Spec.NodeName,
	}) {
		// 立即删除 pod
		if err := podHandler.DeleteMandatory(ctx, pod, statefulPod); err != nil {
			return false
		}
		if statefulPod.Spec.PVCTemplate!=nil{
			if obj,ok:=pvcHandler.IsExists(ctx,types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      *pvcHandler.GetName(statefulPod,*index),
			});ok{
				pvc:=obj.(*corev1.PersistentVolumeClaim)
				if err:=pvcHandler.DeleteMandatory(ctx,pvc,statefulPod);err!=nil{
					return false
				}
			}
		}
		statefulPod.Status.PodStatusMes[*index].Status = Deleting
		statefulPod.Status.PVCStatusMes[*index].Status = pvc_controller.Deleting
		return true
	}

	// pod running
	if pod.Status.Phase == corev1.PodRunning {
		if statefulPod.Status.PodStatusMes[*index].Status == corev1.PodRunning {
			return false
		}
		statefulPod.Status.PodStatusMes[*index].PodName = pod.Name
		statefulPod.Status.PodStatusMes[*index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[*index].NodeName = pod.Spec.NodeName
		return true
	}
	return false
}
