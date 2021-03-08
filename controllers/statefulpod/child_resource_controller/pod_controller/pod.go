package pod_controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	resourcecfg "github.com/q8s-io/iapetos/initconfig"
	nodeservice "github.com/q8s-io/iapetos/services/node"
	podservice "github.com/q8s-io/iapetos/services/pod"
	pvservice "github.com/q8s-io/iapetos/services/pv"
	pvcservice "github.com/q8s-io/iapetos/services/pvc"
)

type PodCtrl struct {
	client.Client
}

type PodCtrlFunc interface {
	ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error)
	ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (bool, error)
	DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error
	MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int
	MonitorPodStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod, index *int) bool
}

func NewPodCtrl(client client.Client) PodCtrlFunc {
	return &PodCtrl{client}
}

// 扩容 pod
func (podctrl *PodCtrl) ExpansionPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PodStatus, error) {
	podHandler := podservice.NewPodService(podctrl.Client)
	podName := fmt.Sprintf("%v-%v", statefulPod.Name, index)
	podIndex := int32(index)

	if _, err, ok := podHandler.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); !ok && err == nil { // pod 不存在，创建 pod
		pod := podHandler.PodTempale(ctx, statefulPod, podName, index)
		if err := podHandler.CreatePod(ctx, pod); err != nil {
			return nil, err
		}
		podStatus := &iapetosapiv1.PodStatus{
			PodName: pod.Name,
			Status:  podservice.Preparing,
			Index:   &podIndex,
		}
		return podStatus, nil
		// pod 存在，podStatus 不变
	} else if ok && err == nil {
		podStatus := statefulPod.Status.PodStatusMes[index]
		return &podStatus, err
	} else {
		return nil, err
	}
}

// 缩容 pod
func (podctrl *PodCtrl) ShrinkPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (bool, error) {
	podName := fmt.Sprintf("%v-%v", statefulPod.Name, index-1)
	podHandler := podservice.NewPodService(podctrl.Client)

	if pod, err, ok := podHandler.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok { // 若 pod 正在被删除则不再删除
		if podHandler.JudgmentPodDel(pod) {
			return false, nil
		}
		if err := podHandler.DeletePod(ctx, pod); err != nil {
			return false, err
		}
		// pod 删除完毕
	} else if err == nil && !ok {
		return true, nil
	} else {
		return false, err
	}

	return false, nil
}

func (podctrl *PodCtrl) DeletePodAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error {
	podHandler := podservice.NewPodService(podctrl.Client)
	pvcHandler := pvcservice.NewPVCService(podctrl.Client)
	pvHandler := pvservice.NewPVService(podctrl.Client)

	for i, v := range statefulPod.Status.PodStatusMes {
		if pod, err, ok := podHandler.IsPodExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PodName,
		}); err == nil && ok { // pod 存在，删除 pod
			if err := podHandler.DeletePod(ctx, pod); err != nil {
				return err
			}
			pvcName := pvcHandler.SetPVCName(statefulPod, i)
			if pvc, err, ok := pvcHandler.IsPVCExist(ctx, types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      pvcName,
				// pvc 存在，删除 pvc
			}); err == nil && ok {
				// 判断 pv 策略是否为回收策略
				if statefulPod.Spec.PVRecyclePolicy == corev1.PersistentVolumeReclaimRetain {
					pvName := pvc.Spec.VolumeName
					if err := pvHandler.SetPVRetain(ctx, &pvName); err != nil { // 将 pv 策略设置为回收策略
						return err
					}
					// 删除 pvc
					if err := pvcHandler.DeletePVC(ctx, pvc); err != nil {
						return err
					}
					// 等待 pvc 删除完毕
					for {
						if _, err, ok := pvcHandler.IsPVCExist(ctx, types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						}); err == nil && !ok { // pvc 删除完毕，退出
							break
						}
					}
					// 将 pv 设置为可用
					if err := pvHandler.SetVolumeAvailable(ctx, &pvName); err != nil {
						return err
					}
				} else {
					// 直接删除 pvc
					if err := pvcHandler.DeletePVC(ctx, pvc); err != nil {
						return err
					}
				}
			} else if err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (podctrl *PodCtrl) MaintainPod(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) *int {
	podHandler := podservice.NewPodService(podctrl.Client)
	for i, pod := range statefulPod.Status.PodStatusMes {
		// 如果 statefulPod.status.podstatusmes 的状态为 deleting，pod 不存在，返回 pod 索引
		if pod.Status == podservice.Deleting {
			if _, err, ok := podHandler.IsPodExist(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); err == nil && !ok {
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
	nodeHandler := nodeservice.NewNodeService(podctrl.Client)
	pvcHandler := pvcservice.NewPVCService(podctrl.Client)

	if podHandler.JudgmentPodDel(pod) {
		// 设置过 deleting 状态则不再进行设置
		if statefulPod.Status.PodStatusMes[*index].Status == podservice.Deleting {
			return false
		}
		statefulPod.Status.PodStatusMes[*index].Status = podservice.Deleting
		return true
	}

	// node Unhealthy
	if !nodeHandler.IsNodeReady(ctx, pod.Spec.NodeName) {
		// pod 是否需要删除
		if podHandler.JudgmentPodDel(pod) {
			return false
		}
		// 立即删除 pod
		if err := podHandler.DeletePodMandatory(ctx, pod, statefulPod); err != nil {
			return false
		} else {
			statefulPod.Status.PodStatusMes[*index].Status = podservice.Deleting
			statefulPod.Status.PVCStatusMes[*index].Status = pvcservice.Deleting
			return true
		}
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

	// create pod timeout
	if pod.Status.Phase != corev1.PodRunning && time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) {
		if err := podHandler.DeletePod(ctx, pod); err != nil {
			return false
		}
		pvcName := pvcHandler.SetPVCName(statefulPod, *index)
		if pvc, err, ok := pvcHandler.IsPVCExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      pvcName,
		}); err == nil && ok { // pvc 存在，删除 pvc
			if err := pvcHandler.DeletePVC(ctx, pvc); err != nil {
				return false
			}
		} else if err != nil {
			return false
		}
		// 维护时重新拉起 pod 超时
		if len(statefulPod.Status.PodStatusMes) == int(*statefulPod.Spec.Size) || *index==int(*statefulPod.Spec.Size){
			statefulPod.Status.PodStatusMes[*index].Status = podservice.Deleting
			statefulPod.Status.PVCStatusMes[*index].Status = pvcservice.Deleting
			// 扩容时创建 pod 超时
		} else {
			statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:*index]
			statefulPod.Status.PVCStatusMes = statefulPod.Status.PVCStatusMes[:*index]
			newIndex:=*index-1
			index=&newIndex
		}
		return true
	}

	return false
}
