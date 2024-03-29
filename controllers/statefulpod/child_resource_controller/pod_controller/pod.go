package pod_controller

import (
	"context"
	"errors"
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

const (
	Preparing     = corev1.PodPhase("Preparing")
	Deleting      = corev1.PodPhase("Deleting")
	CreateTimeOut = corev1.PodPhase("CreateTimeOut")
	//TimeOutIndex="TimeOutIndex"
)

type PodCtrlFunc interface {
	ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error)
	ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
	DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool
	MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int
	MonitorPodStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod, index *int) bool
	PodIsOk(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int
	//IsCreationPodTimeout(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
	IsPodDeleting(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
	//CodbPodReady(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(error)
}

func NewPodCtrl(client client.Client) PodCtrlFunc {
	return &PodCtrl{client}
}

func (podctrl *PodCtrl) IsPodDeleting(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool {
	if index <= 0 {
		return false
	}
	index--
	podHandler := podservice.NewPodService(podctrl.Client)
	if obj, ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podHandler.GetName(statefulPod, index),
	}); ok {
		lastPod := obj.(*corev1.Pod)
		if !lastPod.DeletionTimestamp.IsZero() {
			return true
		}
	} else {
		return true
	}
	return false
}

// 扩容 pod
func (podctrl *PodCtrl) ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error) {
	podHandler := podservice.NewPodService(podctrl.Client)
	podName := podHandler.GetName(statefulPod, index)
	podIndex := int32(index)
	if _, ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podName,
	}); !ok { // pod 不存在，创建 pod
		podTemplate := podHandler.CreateTemplate(ctx, statefulPod, *podName, index)
		obj, err := podHandler.Create(ctx, podTemplate)
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
	} else {
		if index >= len(statefulPod.Status.PodStatusMes) {
			return nil, errors.New("")
		}
		podStatus := statefulPod.Status.PodStatusMes[index]
		return &podStatus, nil
	}
}

// 缩容 pod
func (podctrl *PodCtrl) ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool {
	podHandler := podservice.NewPodService(podctrl.Client)
	podName := podHandler.GetName(statefulPod, index)
	if pod, ok := podHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *podName,
	}); ok {
		if err := podHandler.Delete(ctx, pod); err != nil {
			return false
		}
		// pod 删除完毕
	} else {
		return true
	}
	return false
}

func (podctrl *PodCtrl) DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool {
	podHandler := podservice.NewPodService(podctrl.Client)
	sum := 0
	for _, v := range statefulPod.Status.PodStatusMes {
		if pod, ok := podHandler.IsExists(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PodName,
		}); ok { // pod 存在，删除 pod
			if err := podHandler.Delete(ctx, pod); err != nil {
				return false
			}
		} else {
			sum++
		}
	}
	if sum == len(statefulPod.Status.PVCStatusMes) {
		return true
	}
	return false
}

func (podctrl *PodCtrl) MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int {
	podHandler := podservice.NewPodService(podctrl.Client)
	for i, pod := range statefulPod.Status.PodStatusMes {
		// 如果 statefulPod.status.podstatusmes 的状态为 deleting，pod 不存在，返回 pod 索引
		if pod.Status == Deleting {
			if _, ok := podHandler.IsExists(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); !ok {
				return &i
			}
		}
	}
	return nil
}

func (podctrl *PodCtrl) PodIsOk(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int {
	podHandler := podservice.NewPodService(podctrl.Client)
	for i, podMsg := range statefulPod.Status.PodStatusMes {
		if obj, ok := podHandler.IsExists(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      podMsg.PodName,
		}); !ok {
			statefulPod.Status.PodStatusMes[i].Status = Deleting
			return &i
		} else {
			pod := obj.(*corev1.Pod)
			if pod.Status.Phase == corev1.PodRunning && statefulPod.Status.PodStatusMes[i].Status != corev1.PodRunning {
				statefulPod.Status.PodStatusMes[i].Status = corev1.PodRunning
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
	pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	//pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	resourceHandle := services.NewResource(podctrl.Client)
	if !pod.DeletionTimestamp.IsZero() {
		// 设置过 deleting 状态则不再进行设置
		if statefulPod.Status.PodStatusMes[*index].Status == Deleting || statefulPod.Status.PodStatusMes[*index].Status == CreateTimeOut {
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
		if statefulPod.Spec.PVCTemplate != nil {
			if obj, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      *pvcHandler.GetName(statefulPod, *index),
			}); ok {
				pvc := obj.(*corev1.PersistentVolumeClaim)
				if err := pvcHandler.DeleteMandatory(ctx, pvc, statefulPod); err != nil {
					return false
				}
			}
		}
		statefulPod.Status.PodStatusMes[*index].Status = Deleting
		statefulPod.Status.PVCStatusMes[*index].Status = pvc_controller.Deleting
		return true
	}

	// pod running
	if podctrl.isPodRunning(pod) {
		if statefulPod.Status.PodStatusMes[*index].Status == corev1.PodRunning {
			return false
		}
		statefulPod.Status.PodStatusMes[*index].PodName = pod.Name
		statefulPod.Status.PodStatusMes[*index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[*index].NodeName = pod.Spec.NodeName
		return true
	}

	if statefulPod.Status.PodStatusMes[*index].Status == CreateTimeOut {
		if err := podHandler.Delete(ctx, pod); err != nil {
			return false
		}
		if statefulPod.Spec.PVCTemplate!=nil{
			if obj, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
				Namespace: pod.Namespace,
				Name:      *pvcHandler.GetName(statefulPod, *index),
			}); ok {
				pvc := obj.(*corev1.PersistentVolumeClaim)
			DELETEPVC: // 等待pvc删除完毕，这里不是幂等关系，一次一定要等pvc删除成功
				if err := pvcHandler.Delete(ctx, pvc); err != nil {
					goto DELETEPVC
				}
			}
		}
		// 初始化创建时超时
		if *index == len(statefulPod.Status.PodStatusMes)-1 {
			statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:*index]
			statefulPod.Status.PVCStatusMes = statefulPod.Status.PVCStatusMes[:*index]
		} else { // 维护创建时超时
			statefulPod.Status.PodStatusMes[*index].Status = Deleting
			statefulPod.Status.PVCStatusMes[*index].Status = pvc_controller.Deleting
		}
		return true
	}
	// pod创建超时
	if !podctrl.isPodRunning(pod) && time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) {
		statefulPod.Status.PodStatusMes[*index].Status = CreateTimeOut
		return true
	}
	return false
}

// pod 内所有的pod都是 running 和 ready 状态
func (podctrl *PodCtrl)isPodRunning(pod *corev1.Pod)bool{
	if pod.Status.Phase!=corev1.PodRunning{
		return false
	}
	for _,v:=range pod.Status.Conditions{
		if v.Type==corev1.PodReady && v.Status!=corev1.ConditionTrue{
			return false
		}
	}
	return true
}
