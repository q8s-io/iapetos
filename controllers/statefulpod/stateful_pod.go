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

func (s *StatefulPodController) changeStatefulPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	for {
		if err := s.Update(ctx, statefulPod, client.DryRunAll); err == nil {
			if err := s.Update(ctx, statefulPod); err != nil {
				return err
			}
			break
		} else {
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
		if pod.PodName == "" {
			return s.expansionPod(ctx, statefulPod, int(*pod.Index))
		}
		if pod.Status == podcontrl.Deleting {
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
func (s *StatefulPodController) expansionPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	podIndex := int32(index)
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index)
	if _, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); !ok && err == nil {
		pod := podcontrl.NewPodController(s.Client).PodTempale(ctx, statefulPod, podName, index)
		if err := podcontrl.NewPodController(s.Client).CreatePod(ctx, pod); err != nil {
			return err
		}
		var pvcStatus statefulpodv1.PvcStatus
		if statefulPod.Spec.PvcTemplate != nil {
			if ok, err := s.expansionPvc(ctx, statefulPod, index); err != nil {
				return err
			} else if ok {
				pvcStatus = statefulpodv1.PvcStatus{
					Index:        &podIndex,
					PvcName:      pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName,
					Status:       corev1.ClaimPending,
					AccessModes:  statefulPod.Spec.PvcTemplate.AccessModes,
					StorageClass: *statefulPod.Spec.PvcTemplate.StorageClassName,
				}
			} else if !ok {
				pvcStatus = statefulPod.Status.PvcStatusMes[index]
			}
		} else {
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

// 缩容
func (s *StatefulPodController) shrinkPod(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) error {
	podName := fmt.Sprintf("%v%v", statefulPod.Name, index-1)
	if pod, err, ok := podcontrl.NewPodController(s.Client).IsPodExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      podName,
	}); err == nil && ok {
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
			return nil
		}
		if err := podcontrl.NewPodController(s.Client).DeletePod(ctx, pod); err != nil {
			return err
		}
	} else if err == nil && !ok {
		return s.shrinkPvc(ctx, statefulPod, index)
	}
	return nil
}

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
		if !tools.ContainsString(statefulPod.Finalizers, myFinalizerName) {
			statefulPod.Finalizers = append(statefulPod.Finalizers, myFinalizerName)
			if err := s.changeStatefulPod(ctx, statefulPod, 0); err != nil {
				return err
			}
		}
	} else {
		if tools.ContainsString(statefulPod.Finalizers, myFinalizerName) {
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
		if podcontrl.NewPodController(s.Client).JudgmentPodDel(pod) {
			return nil
		}
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

	}
	return nil
}

// pvc 扩容
func (s *StatefulPodController) expansionPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error) {
	pvcName := pvccontrl.NewPvcController(s.Client).SetPvcName(statefulPod, index)
	if _, err, ok := pvccontrl.NewPvcController(s.Client).IsPvcExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && !ok {
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
	}); err == nil && ok {
		if !pvc.DeletionTimestamp.IsZero() {
			return nil
		}
		if err := pvccontrl.NewPvcController(s.Client).DeletePVC(ctx, pvc); err != nil {
			return err
		}
	} else if err == nil && !ok {
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
		statefulPod.Status.PvcStatusMes[index].Capacity = pvc.Spec.Resources.Requests.StorageEphemeral().String()
		return s.changeStatefulPod(ctx, statefulPod, index)
	}
	return nil
}
