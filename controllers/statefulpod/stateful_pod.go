package statefulpod

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	podctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pod_controller"
	pvcctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pvc_controller"
	svcctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/service_controller"
	"github.com/q8s-io/iapetos/tools"
)

const (
	ParentNmae = "parentName"
)

type StatefulPodCtrl struct {
	client.Client
}

type StatefulPodCtrlFunc interface {
	CoreCtrl(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error
	MonitorPodStatus(ctx context.Context, pod *corev1.Pod) error
	MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error
}

func NewStatefulPodCtrl(client client.Client) StatefulPodCtrlFunc {
	return &StatefulPodCtrl{client}
}

// StatefulPod 控制器
// len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) 扩容
// len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) 缩容
// len(statefulPod.Status.PodStatusMes) == int(*statefulPod.Spec.Size) 设置 Finalizer，维护
func (s *StatefulPodCtrl) CoreCtrl(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error {
	lenStatus := len(statefulPod.Status.PodStatusMes)

	// 删除中
	if !statefulPod.DeletionTimestamp.IsZero() {
		myFinalizerName := iapetosapiv1.GroupVersion.String()
		// 删除 statefulPod
		if tools.MatchStringFromArray(statefulPod.Finalizers, myFinalizerName) {
			// 删除 pod、pvc
			if err := podctrl.NewPodCtrl(s.Client).DeletePodAll(ctx, statefulPod); err != nil {
				return err
			}
			// 移除 service 的 finalizer
			if err := svcctrl.NewServiceController(s.Client).RemoveServiceFinalizer(ctx, statefulPod); err != nil {
				return err
			}
			statefulPod.Finalizers = tools.RemoveString(statefulPod.Finalizers, myFinalizerName)
			if err := s.updateStatefulPodStatus(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
		return nil
	}

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

// 修改 statefulSet 状态，index 代表 podStatus、pvcStatus 要修改的索引位置
func (s *StatefulPodCtrl) updateStatefulPodStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) error {
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
			newStatefulPod.Status.PVCStatusMes[index] = statefulPod.Status.PVCStatusMes[index]
			statefulPod = newStatefulPod
		}
	}
	return nil
}

// 扩容
// 创建 service
// pvc 需要创建，则创建 pvc，不需要 pvcStatus 设置为 0 值
// index == len(statefulPod.Status.PodStatusMes) 代表创建
// index != len(statefulPod.Status.PodStatusMes) 代表维护
func (s *StatefulPodCtrl) expansion(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) error {
	defer func() {
		if recover() != nil {
			return
		}
	}()

	var podStatus *iapetosapiv1.PodStatus
	var pvcStatus *iapetosapiv1.PVCStatus
	var err error

	serviceCtrl := svcctrl.NewServiceController(s.Client)
	podCtrl := podctrl.NewPodCtrl(s.Client)
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)

	// 索引为 0，且需要生成 service
	if index == 0 && statefulPod.Spec.ServiceTemplate != nil {
		if ok, err := serviceCtrl.CreateService(ctx, statefulPod); err != nil {
			return err
		} else if !ok { // service 未创建
			return nil
		}
	}

	if podStatus, err = podCtrl.ExpansionPod(ctx, statefulPod, index); err != nil {
		return err
	}

	if statefulPod.Spec.PVCTemplate != nil {
		if pvcStatus, err = pvcCtrl.ExpansionPVC(ctx, statefulPod, index); err != nil {
			return err
		}
	} else {
		pvcStatus = &iapetosapiv1.PVCStatus{
			Index:       tools.IntToIntr32(index),
			PVCName:     "none",
			Status:      "",
			AccessModes: []corev1.PersistentVolumeAccessMode{"none"},
		}
	}

	if len(statefulPod.Status.PodStatusMes) == index {
		statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, *podStatus)
		statefulPod.Status.PVCStatusMes = append(statefulPod.Status.PVCStatusMes, *pvcStatus)
	} else {
		statefulPod.Status.PodStatusMes[index] = *podStatus
		statefulPod.Status.PVCStatusMes[index] = *pvcStatus
	}

	return s.updateStatefulPodStatus(ctx, statefulPod, index)
}

// 缩容
// 若 pvc 存在，删除 pvc
func (s *StatefulPodCtrl) shrink(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) error {
	podCtrl := podctrl.NewPodCtrl(s.Client)
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)

	// 判断 pod 是否删除完毕
	if ok, err := podCtrl.ShrinkPod(ctx, statefulPod, index); err != nil {
		return err
	} else if !ok {
		return nil
	}

	// 判断 pvc 是否删除完毕
	if ok, err := pvcCtrl.ShrinkPVC(ctx, statefulPod, index); err != nil {
		return err
	} else if !ok {
		return nil
	}

	statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
	statefulPod.Status.PVCStatusMes = statefulPod.Status.PVCStatusMes[:index-1]

	return s.updateStatefulPodStatus(ctx, statefulPod, index-1)
}

// 维护 pod 状态
func (s *StatefulPodCtrl) maintain(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error {
	podCtrl := podctrl.NewPodCtrl(s.Client)

	if index := podCtrl.MaintainPod(ctx, statefulPod); index != nil {
		return s.expansion(ctx, statefulPod, *index)
	}

	return nil
}

// 根据 namespace name 获取 statefulPod
func (s *StatefulPodCtrl) getStatefulPod(ctx context.Context, namespaceName *types.NamespacedName) *iapetosapiv1.StatefulPod {
	var statefulPod iapetosapiv1.StatefulPod
	if err := s.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}, &statefulPod); err != nil {
		return nil
	}
	return &statefulPod
}

// 设置 statefulPod finalizer
func (s *StatefulPodCtrl) setFinalizer(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error {
	myFinalizerName := iapetosapiv1.GroupVersion.String()
	if statefulPod.ObjectMeta.DeletionTimestamp.IsZero() {
		if !tools.MatchStringFromArray(statefulPod.Finalizers, myFinalizerName) { // finalizer未设置，则添加finalizer
			statefulPod.Finalizers = append(statefulPod.Finalizers, myFinalizerName)
			if err := s.updateStatefulPodStatus(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// 处理 pod 不同的 status
// pod 异常退出，重新拉起 pod
// node 节点失联，新建 pod、pvc
// pod running 状态，修改 statefulPod.status.PodStatusMes
// pod 创建超时，删除重新创建
func (s *StatefulPodCtrl) MonitorPodStatus(ctx context.Context, pod *corev1.Pod) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StringToInt(pod.Annotations["index"])
	podctl := podctrl.NewPodCtrl(s.Client)
	if ok := podctl.MonitorPodStatus(ctx, statefulPod, pod, index); ok {
		return s.updateStatefulPodStatus(ctx, statefulPod, index)
	}
	return nil
}

// 处理 pvc 不同的 status
func (s *StatefulPodCtrl) MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      pvc.Annotations[ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StringToInt(pvc.Annotations["index"])
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)
	if ok := pvcCtrl.MonitorPVCStatus(ctx, statefulPod, pvc, index); ok {
		return s.updateStatefulPodStatus(ctx, statefulPod, index)
	}
	return nil
}
