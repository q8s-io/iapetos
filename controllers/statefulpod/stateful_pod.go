package statefulpod

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	nodecontrl "iapetos/controllers/node"
	podcontrl "iapetos/controllers/pod"
	pvccontrl "iapetos/controllers/pvc"
	servicecontrl "iapetos/controllers/service"
	resourcecfg "iapetos/initconfig"
	"iapetos/tools"
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
	if len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) {
		return s.expansionPod(ctx, statefulPod, lenStatus)
	} else if len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) {
		return s.shrinkPod(ctx, statefulPod, lenStatus)
	} else {
		if err := s.setFinalizer(ctx, statefulPod); err != nil {
			return err
		}
		return s.maintainPod(ctx, statefulPod)
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
func (s *StatefulPodController) maintainPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	for i, pod := range statefulPod.Status.PodStatusMes {
		if pod.PodName == "" { // 若 podName 为空，代表pod异常退出，需要重新拉起
			return s.expansionPod(ctx, statefulPod, int(*pod.Index))
		}
		if pod.Status == podcontrl.Deleting { // 如果 statefulPod.status.podstatusmes的状态为deleting,
			// 将 statefulPod.status.podstatusmes 对应索引的 pod name 设置为空
			if _, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{Namespace: statefulPod.Namespace, Name: pod.PodName}); err == nil && !ok {
				statefulPod.Status.PodStatusMes[i] = statefulpodv1.PodStatus{
					PodName:  "",
					Status:   "",
					Index:    statefulPod.Status.PodStatusMes[i].Index,
					NodeName: "",
				}
				return s.changeStatefulPod(ctx, statefulPod, i)
			}
		}
	}
	return nil
}

// 扩容
// 若index ==0 ,创建 service
// 若 pvc 需要创建，则创建pvc ,不需要pvcStatus设置为0值
// 若 index == len(statefulPod.Status.PodStatusMes) 代表创建
// 若 index != len(statefulPod.Status.PodStatusMes) 代表维护
func (s *StatefulPodController) expansionPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	if index == 0 && statefulPod.Spec.ServiceTemplate != nil { // 索引为 0，且需要生成 service
		if ok, err := s.createService(ctx, statefulPod); err != nil {
			return err
		} else if !ok { // service 未创建
			return nil
		}
	}
	podIndex := int32(index)
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index)
	if _, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); !ok && err == nil { // pod 不存在，创建pod
		pod := podcontrl.NewPodController(s.Client).PodTempale(ctx, statefulPod, podName, index)
		if err := podcontrl.NewPodController(s.Client).CreatePod(ctx, pod); err != nil {
			return err
		}
		var pvcStatus statefulpodv1.PvcStatus
		if statefulPod.Spec.PvcTemplate != nil { // 需要创建 pvc
			if ok, err := s.expansionPvc(ctx, statefulPod, index); err != nil {
				return err
			} else if ok { // pvc 不存在，将 statefulPod.Spec.PvcTemplate的 进行设置
				pvcStatus = statefulpodv1.PvcStatus{
					Index:        &podIndex,
					PvcName:      pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName,
					Status:       corev1.ClaimPending,
					AccessModes:  statefulPod.Spec.PvcTemplate.AccessModes,
					StorageClass: *statefulPod.Spec.PvcTemplate.StorageClassName,
				}
			} else if !ok { // pvc存在,statefulPod.Spec.PvcTemplate保持不变
				pvcStatus = statefulPod.Status.PvcStatusMes[index]
			}
		} else { // 不需要；创建 pvc ，将statefulPod.Spec.PvcTemplate的数据进行初始化
			pvcStatus = statefulpodv1.PvcStatus{
				Index:       &podIndex,
				PvcName:     "none",
				Status:      "",
				AccessModes: []corev1.PersistentVolumeAccessMode{"none"},
			}
		}
		if len(statefulPod.Status.PodStatusMes) == index {
			statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, statefulpodv1.PodStatus{
				PodName: pod.Name,
				Status:  podcontrl.Preparing,
				Index:   &podIndex,
			})
			statefulPod.Status.PvcStatusMes = append(statefulPod.Status.PvcStatusMes, pvcStatus)
		} else {
			statefulPod.Status.PodStatusMes[index] = statefulpodv1.PodStatus{
				PodName:  pod.Name,
				Status:   podcontrl.Preparing,
				Index:    &podIndex,
				NodeName: "",
			}
			statefulPod.Status.PvcStatusMes[index] = pvcStatus
		}
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	return nil
}

// 缩容 若pvc存在 ,删除pvc
func (s *StatefulPodController) shrinkPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index-1)
	if pod, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok {
		// 若pod 已经删除过则不再删除
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
			return nil
		}
		if err := podcontrl.NewPodController(s.Client).DeletePod(ctx, pod); err != nil {
			return err
		}
	} else if err == nil && !ok {
		// pod 删除完毕 删除 pvc
		return s.shrinkPvc(ctx, statefulPod, index)
	}
	return nil
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
			statefulPod.Finalizers = tools.RemoveString(statefulPod.Finalizers, myFinalizerName)
			if err := s.changeStatefulPod(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// 处理 pod 不同的 status
func (s *StatefulPodController) MonitorPodStatus(ctx context.Context, pod *corev1.Pod) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[podcontrl.ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StrToInt(pod.Annotations["index"])
	// pod delete
	if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
		// 设置过 deleting状态则不再进行设置
		if statefulPod.Status.PodStatusMes[index].Status == podcontrl.Deleting {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].Status = podcontrl.Deleting
		if err := s.changeStatefulPod(ctx, statefulPod, index); err != nil {
			return err
		}
		return nil
	}

	// node Unhealthy
	if !nodecontrl.NewNodeContrl(s.Client).IsNodeReady(ctx, pod.Spec.NodeName) {
		// pod 是否需要删除
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
			return nil
		}
		// 立即删除 pod
		if err := podcontrl.NewPodController(s.Client).DeletePodMandatory(ctx, pod, statefulPod); err != nil {
			return err
		} else {
			statefulPod.Status.PodStatusMes[index].Status = podcontrl.Deleting
			statefulPod.Status.PvcStatusMes[index].Status = pvccontrl.Deleting
			if err := s.changeStatefulPod(ctx, statefulPod, index); err != nil {
				return err
			}
		}
		return nil
	}

	// pod running
	if pod.Status.Phase == corev1.PodRunning {
		if statefulPod.Status.PodStatusMes[index].Status == corev1.PodRunning {
			return nil
		}
		statefulPod.Status.PodStatusMes[index].PodName = pod.Name
		statefulPod.Status.PodStatusMes[index].Status = corev1.PodRunning
		statefulPod.Status.PodStatusMes[index].NodeName = pod.Spec.NodeName
		if err := s.changeStatefulPod(ctx, statefulPod, index); err != nil {
			return err
		}
	}

	// create pod timeout
	if time.Now().Sub(pod.CreationTimestamp.Time) >= time.Second*time.Duration(resourcecfg.StatefulPodResourceCfg.Pod.Timeout) && pod.Status.Phase != corev1.PodRunning {
		if err := podcontrl.NewPodController(s.Client).DeletePod(ctx, pod); err != nil {
			return err
		}
		pvcName := pvccontrl.NewPvcController(s.Client).SetPvcName(statefulPod, index)
		if pvc, err, ok := pvccontrl.NewPvcController(s.Client).IsPvcExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      pvcName,
		}); err == nil && ok { // pvc 存在，删除 pvc
			if err := pvccontrl.NewPvcController(s.Client).DeletePVC(ctx, pvc); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		// 跟新状态，进行重建
		statefulPod.Status.PodStatusMes[index].Status = podcontrl.Deleting
		statefulPod.Status.PvcStatusMes[index].Status = pvccontrl.Deleting
		// 重新拉取 pod pvc
		if err := s.expansionPod(ctx, statefulPod, index); err != nil {
			return err
		}
		if err := s.changeStatefulPod(ctx, statefulPod, index); err != nil {
			return err
		}
	}
	return nil
}

// pvc 扩容
func (s *StatefulPodController) expansionPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error) {
	pvcName := pvccontrl.NewPvcController(s.Client).SetPvcName(statefulPod, index)
	if _, err, ok := pvccontrl.NewPvcController(s.Client).IsPvcExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && !ok { // pvc 不存在。创建 pvc
		pvcTemplate, _ := pvccontrl.NewPvcController(s.Client).PvcTemplate(ctx, statefulPod, pvcName, index)
		if err := pvccontrl.NewPvcController(s.Client).CreatePVC(ctx, pvcTemplate); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

// pvc 缩容
func (s *StatefulPodController) shrinkPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	pvcName := pvccontrl.NewPvcController(s.Client).SetPvcName(statefulPod, index-1)
	if pvc, err, ok := pvccontrl.NewPvcController(s.Client).IsPvcExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && ok { // pvc 存在，删除 pvc
		if !pvc.DeletionTimestamp.IsZero() { // pvc 正在删除删除
			return nil
		}
		if err := pvccontrl.NewPvcController(s.Client).DeletePVC(ctx, pvc); err != nil {
			return err
		}
	} else if err == nil && !ok { // pvc 删除完毕,statefulPod.Status.PodStatusMes长度减一
		statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
		statefulPod.Status.PvcStatusMes = statefulPod.Status.PvcStatusMes[:index-1]
		if err := s.changeStatefulPod(ctx, statefulPod, index-1); err != nil {
			return err
		}
	}
	return nil
}

// 处理 pvc 不同的 status
func (s *StatefulPodController) MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error {
	statefulPod := s.getStatefulPod(ctx, &types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      pvc.Annotations[podcontrl.ParentNmae],
	})
	if statefulPod == nil {
		return nil
	}
	index := tools.StrToInt(pvc.Annotations["index"])
	if !pvc.DeletionTimestamp.IsZero() {
		if statefulPod.Status.PvcStatusMes[index].Status == pvccontrl.Deleting {
			return nil
		}
		statefulPod.Status.PvcStatusMes[index].Status = pvccontrl.Deleting
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	if pvc.Status.Phase == corev1.ClaimBound {
		if statefulPod.Status.PvcStatusMes[index].Status == corev1.ClaimBound {
			return nil
		}
		statefulPod.Status.PvcStatusMes[index].Status = corev1.ClaimBound
		capicity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		statefulPod.Status.PvcStatusMes[index].Capacity = capicity.String()
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	return nil
}

// 创建 service
func (s *StatefulPodController) createService(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) (bool, error) {
	serviceName := fmt.Sprintf("%v-%v", statefulPod.Name, "service")
	if _, err, ok := servicecontrl.NewServiceContrl(s.Client).IsServiceExits(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      serviceName,
	}); err == nil && !ok { // service 不存在 ,创建 service
		serviceTemplate := servicecontrl.NewServiceContrl(s.Client).ServiceTemplate(statefulPod)
		if err := servicecontrl.NewServiceContrl(s.Client).CreateService(ctx, serviceTemplate); err != nil {
			return false, err
		}
	} else if err == nil && ok {
		return true, nil
	}
	return false, nil
}
