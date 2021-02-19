package statefulpod

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	podcontrl "iapetos/controllers/statefulpod/child_resource_controller/pod_controller"
	pvccontrl "iapetos/controllers/statefulpod/child_resource_controller/pvc_controller"
	servicecontrl "iapetos/controllers/statefulpod/child_resource_controller/service_controller"
	"iapetos/tools"
)

const (
	ParentNmae = "parentName"
)

type StatefulPodController struct {
	client.Client
}

type StatefulPodContrlInf interface {
	StatefulPodContrl(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error
	MonitorPodStatus(ctx context.Context, pod *corev1.Pod) error
	MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error
}

func NewStatefulPodController(client client.Client) StatefulPodContrlInf {
	return &StatefulPodController{client}
}

// statefulPod 控制器
// 若 len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) 扩容
// 若 len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) 缩容
// 若 len(statefulPod.Status.PodStatusMes) == int(*statefulPod.Spec.Size) 设置Finalizer,维护
func (s *StatefulPodController) StatefulPodContrl(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	lenStatus := len(statefulPod.Status.PodStatusMes)
	if lenStatus < int(*statefulPod.Spec.Size) {
		return s.expansion(ctx, statefulPod, lenStatus)
	} else if lenStatus > int(*statefulPod.Spec.Size) {
		return s.shrink(ctx, statefulPod, lenStatus)
	} else {
		if err := s.setFinalizer(ctx, statefulPod); err != nil {
			return err
		}
		return s.maintain(ctx, statefulPod)
	}
}

// 修改 statefulSet 状态,index 代表podStatus pvcStatus 要修改的索引位置
func (s *StatefulPodController) changeStatefulPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	for {
		if err := s.Update(ctx, statefulPod, client.DryRunAll); err == nil {
			if err := s.Update(ctx, statefulPod); err != nil {
				return err
			}
			break
		} else {
			// 更新失败，拉取最新资源
			newStatefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      statefulPod.Name,
			})
			if newStatefulPod == nil {
				break
			}
			newStatefulPod.Status.PodStatusMes[index] = statefulPod.Status.PodStatusMes[index]
			newStatefulPod.Status.PvcStatusMes[index] = statefulPod.Status.PvcStatusMes[index]
			statefulPod = newStatefulPod
		}
	}
	return nil
}

// 维护pod状态
func (s *StatefulPodController) maintain(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	podctl := podcontrl.NewPodController(s.Client)
	if index := podctl.MaintainPod(ctx, statefulPod); index != nil {
		return s.expansion(ctx, statefulPod, *index)
	}
	return nil
}

// 扩容
// 创建 service
// 若 pvc 需要创建，则创建pvc ,不需要pvcStatus设置为0值
// 若 index == len(statefulPod.Status.PodStatusMes) 代表创建
// 若 index != len(statefulPod.Status.PodStatusMes) 代表维护
func (s *StatefulPodController) expansion(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	servicectl := servicecontrl.NewServiceController(s.Client)
	podctl := podcontrl.NewPodController(s.Client)
	pvcctl := pvccontrl.NewPvcController(s.Client)
	var podstatus *statefulpodv1.PodStatus
	var pvcstatus *statefulpodv1.PvcStatus
	var err error
	if index == 0 && statefulPod.Spec.ServiceTemplate != nil { // 索引为 0，且需要生成 service
		if ok, err := servicectl.CreateService(ctx, statefulPod); err != nil {
			return err
		} else if !ok { // service 未创建
			return nil
		}
	}
	if podstatus, err = podctl.ExpansionPod(ctx, statefulPod, index); err != nil {
		return err
	}
	if statefulPod.Spec.PvcTemplate != nil {
		if pvcstatus, err = pvcctl.ExpansionPvc(ctx, statefulPod, index); err != nil {
			return err
		}
	} else {
		pvcstatus = &statefulpodv1.PvcStatus{
			Index:       tools.IntToIntr32(index),
			PvcName:     "none",
			Status:      "",
			AccessModes: []corev1.PersistentVolumeAccessMode{"none"},
		}
	}
	if len(statefulPod.Status.PodStatusMes) == index {
		statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, *podstatus)
		statefulPod.Status.PvcStatusMes = append(statefulPod.Status.PvcStatusMes, *pvcstatus)
	} else {
		statefulPod.Status.PodStatusMes[index] = *podstatus
		statefulPod.Status.PvcStatusMes[index] = *pvcstatus
	}
	return s.changeStatefulPod(ctx, statefulPod, index)
}

// 缩容 若pvc存在 ,删除pvc
func (s *StatefulPodController) shrink(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	podctl := podcontrl.NewPodController(s.Client)
	pvcctl := pvccontrl.NewPvcController(s.Client)
	if ok, err := podctl.ShrinkPod(ctx, statefulPod, index); err != nil {
		return err
	} else if !ok {
		return nil
	} // 判断 pod 是否删除完毕
	if ok, err := pvcctl.ShrinkPvc(ctx, statefulPod, index); err != nil {
		return err
	} else if !ok {
		return nil
	} // 判断 pvc 是否删除完毕
	statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
	statefulPod.Status.PvcStatusMes = statefulPod.Status.PvcStatusMes[:index-1]
	return s.changeStatefulPod(ctx, statefulPod, index-1)
}

// 根据 namespace name 获取 statefulPod
func (s *StatefulPodController) getStatefulPod(ctx context.Context, namespaceName *types.NamespacedName) *statefulpodv1.StatefulPod {
	var statefulPod statefulpodv1.StatefulPod
	if err := s.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}, &statefulPod); err != nil {
		return nil
	}
	return &statefulPod
}

// 设置 statefulPod finalizer
func (s *StatefulPodController) setFinalizer(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	myFinalizerName := statefulpodv1.GroupVersion.String()
	if statefulPod.ObjectMeta.DeletionTimestamp.IsZero() {
		if !tools.ContainsString(statefulPod.Finalizers, myFinalizerName) { // finalizer未设置，则添加finalizer
			statefulPod.Finalizers = append(statefulPod.Finalizers, myFinalizerName)
			if err := s.changeStatefulPod(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
	} else {
		// 删除 statefulPod
		if tools.ContainsString(statefulPod.Finalizers, myFinalizerName) {
			// 删除 pod ，pvc
			if err := podcontrl.NewPodController(s.Client).DeletePodAll(ctx, statefulPod); err != nil {
				return err
			}
			// 一处 service 的 finalizer
			if err := servicecontrl.NewServiceController(s.Client).RemoveServiceFinalizer(ctx, statefulPod); err != nil {
				return err
			}
			statefulPod.Finalizers = tools.RemoveString(statefulPod.Finalizers, myFinalizerName)
			if err := s.changeStatefulPod(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// 处理 pod 不同的 status
// pod 异常退出，重新拉起 pod
// node 节点失联 新建 pod ,pvc
// pod running 状态，修改 statefulPod.status.PodStatusMes
// pod 创建超时，删除重新创建
func (s *StatefulPodController) MonitorPodStatus(ctx context.Context, pod *corev1.Pod) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StrToInt(pod.Annotations["index"])
	podctl := podcontrl.NewPodController(s.Client)
	if ok := podctl.MonitorPodStatus(ctx, statefulPod, pod, index); ok {
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	return nil
}

// 处理 pvc 不同的 status
func (s *StatefulPodController) MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      pvc.Annotations[ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StrToInt(pvc.Annotations["index"])
	pvcctl := pvccontrl.NewPvcController(s.Client)
	if ok := pvcctl.MonitorPVCStatus(ctx, statefulPod, pvc, index); ok {
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	return nil
}
