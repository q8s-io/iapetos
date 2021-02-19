package pod_controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	nodeservice "iapetos/controllers/node"
	podservice "iapetos/controllers/pod"
	pvservice "iapetos/controllers/pv"
	pvcservice "iapetos/controllers/pvc"
	resourcecfg "iapetos/initconfig"
)

type PodController struct {
	client.Client
}

type PodContrlIntf interface {
	ExpansionPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (*statefulpodv1.PodStatus, error)
	ShrinkPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error)
	DeletePodAll(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error
	MaintainPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) *int
	MonitorPodStatus(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pod *corev1.Pod, index int) bool
}

func NewPodController(client client.Client) PodContrlIntf {
	return &PodController{client}
}

// 扩容 pod
func (podctrl *PodController) ExpansionPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (*statefulpodv1.PodStatus, error) {
	podhandler := podservice.NewPodService(podctrl.Client)
	podIndex := int32(index)
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index)
	if _, err, ok := podhandler.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); !ok && err == nil { // pod 不存在，创建pod
		pod := podhandler.PodTempale(ctx, statefulPod, podName, index)
		if err := podhandler.CreatePod(ctx, pod); err != nil {
			return nil, err
		}
		podStatus := &statefulpodv1.PodStatus{
			PodName: pod.Name,
			Status:  podservice.Preparing,
			Index:   &podIndex,
		}
		return podStatus, nil
	} else if ok && err == nil { // pod 存在，podStatus 不变
		podStatus := statefulPod.Status.PodStatusMes[index]
		return &podStatus, err
	} else {
		return nil, err
	}
}

// 缩容 pod
func (podctrl *PodController) ShrinkPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error) {
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index-1)
	podhandler := podservice.NewPodService(podctrl.Client)
	if pod, err, ok := podhandler.IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok {
		// 若pod 正在被删除则不再删除
		if podhandler.JudgmentPodDel(pod) {
			return false, nil
		}
		if err := podhandler.DeletePod(ctx, pod); err != nil {
			return false, err
		}
	} else if err == nil && !ok {
		// pod 删除完毕
		return true, nil
	} else {
		return false, err
	}
	return false, nil
}

func (podctrl *PodController) DeletePodAll(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	podhandler := podservice.NewPodService(podctrl.Client)
	pvchandler := pvcservice.NewPvcService(podctrl.Client)
	pvhandler := pvservice.NewPVService(podctrl.Client)
	for i, v := range statefulPod.Status.PodStatusMes {
		if pod, err, ok := podhandler.IsPodExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PodName,
		}); err == nil && ok { // pod 存在，删除 pod
			if err := podhandler.DeletePod(ctx, pod); err != nil {
				return err
			}
			pvcName := pvchandler.SetPvcName(statefulPod, i)
			if pvc, err, ok := pvchandler.IsPvcExist(ctx, types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      pvcName,
			}); err == nil && ok { // pvc 存在 ，删除pvc
				if statefulPod.Spec.PVRecyclePolicy == corev1.PersistentVolumeReclaimRetain { // 判断 pv 策略是否为回收策略
					pvName := pvc.Spec.VolumeName
					if err := pvhandler.SetPVRetain(ctx, &pvName); err != nil { // 将 pv 策略设置为回收策略
						return err
					}
					// 删除 pvc
					if err := pvchandler.DeletePVC(ctx, pvc); err != nil {
						return err
					}
					// 等待 pvc 删除完毕
					for {
						if _, err, ok := pvchandler.IsPvcExist(ctx, types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						}); err == nil && !ok { // pvc 删除完毕，退出
							break
						}
					}
					// 将 pv 设置为可用
					if err := pvhandler.SetVolumeAvailable(ctx, &pvName); err != nil {
						return err
					}
				} else {
					// 直接删除 pvc
					if err := pvchandler.DeletePVC(ctx, pvc); err != nil {
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

func (podctrl *PodController) MaintainPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) *int {
	podhandler := podservice.NewPodService(podctrl.Client)
	for i, pod := range statefulPod.Status.PodStatusMes {
		if pod.Status == podservice.Deleting {
			// 如果 statefulPod.status.podstatusmes的状态为deleting,pod 不存在，返回pod索引
			if _, err, ok := podhandler.IsPodExist(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); err == nil && !ok {
				return &i
			}
		}
	}
	return nil
}

func (podctrl *PodController) MonitorPodStatus(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pod *corev1.Pod, index int) bool {
	podhandler := podservice.NewPodService(podctrl.Client)
	nodehandler := nodeservice.NewNodeService(podctrl.Client)
	pvchandler := pvcservice.NewPvcService(podctrl.Client)
	if podhandler.JudgmentPodDel(pod) {
		// 设置过 deleting状态则不再进行设置
		if statefulPod.Status.PodStatusMes[index].Status == podservice.Deleting {
			return false
		}
		statefulPod.Status.PodStatusMes[index].Status = podservice.Deleting
		return true
	}
	// node Unhealthy
	if !nodehandler.IsNodeReady(ctx, pod.Spec.NodeName) {
		// pod 是否需要删除
		if podhandler.JudgmentPodDel(pod) {
			return false
		}
		// 立即删除 pod
		if err := podhandler.DeletePodMandatory(ctx, pod, statefulPod); err != nil {
			return false
		} else {
			statefulPod.Status.PodStatusMes[index].Status = podservice.Deleting
			statefulPod.Status.PvcStatusMes[index].Status = pvcservice.Deleting
			return true
		}
	}
	// pod running
	if pod.Status.Phase == corev1.PodRunning {
		if statefulPod.Status.PodStatusMes[index].Status == corev1.PodRunning {
			return false
		}
		statefulPod.Status.PodStatusMes[index].PodName = pod.Name
		statefulPod.Status.PodStatusMes[index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[index].NodeName = pod.Spec.NodeName
		return true
	}
	// create pod timeout
	if pod.Status.Phase != corev1.PodRunning && time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) {
		if err := podhandler.DeletePod(ctx, pod); err != nil {
			return false
		}
		pvcName := pvchandler.SetPvcName(statefulPod, index)
		if pvc, err, ok := pvchandler.IsPvcExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      pvcName,
		}); err == nil && ok { // pvc 存在，删除 pvc
			if err := pvchandler.DeletePVC(ctx, pvc); err != nil {
				return false
			}
		} else if err != nil {
			return false
		}
		if len(statefulPod.Status.PodStatusMes) == int(*statefulPod.Spec.Size) { // 代表维护时从新拉起pod时超时
			statefulPod.Status.PodStatusMes[index].Status = podservice.Deleting
			statefulPod.Status.PvcStatusMes[index].Status = pvcservice.Deleting
		} else { // 代表扩容时创建pod超时
			statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index]
			statefulPod.Status.PvcStatusMes = statefulPod.Status.PvcStatusMes[:index]
		}
		return true
	}
	return false
}