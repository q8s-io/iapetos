package pod

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	pvcontrl "iapetos/controllers/pv"
	pvccontrl "iapetos/controllers/pvc"
)

const (
	Preparing   = corev1.PodPhase("Preparing")
	Deleting    = corev1.PodPhase("Deleting")
	ParentNmae  = "parentName"
	StatefulPod = "StatefulPod"
	Index       = "index"
)

type PodController struct {
	client.Client
}

type PodContrlIntf interface {
	IsPodExist(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Pod, error, bool)
	PodTempale(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, podName string, index int) *corev1.Pod
	CreatePod(ctx context.Context, pod *corev1.Pod) error
	DeletePod(ctx context.Context, pod *corev1.Pod) error
	DeletePodMandatory(ctx context.Context, pod *corev1.Pod, statefulPod *statefulpodv1.StatefulPod) error
	JudgmentPodDel(pod *corev1.Pod) bool
	DeletePodAll(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error
}

func NewPodController(client client.Client) PodContrlIntf {
	return &PodController{client}
}

func (p *PodController) IsPodExist(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Pod, error, bool) {
	var pod corev1.Pod
	if err := p.Get(ctx, namespaceName, &pod); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		return nil, err, false
	} else {
		return &pod, nil, true
	}
}

func (p *PodController) PodTempale(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, podName string, index int) *corev1.Pod {
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
	if statefulPod.Spec.PvcTemplate != nil {
		pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = pvccontrl.NewPvcController(p.Client).SetPvcName(statefulPod, index)
	}
	if statefulPod.Spec.ServiceTemplate != nil {
		pod.Labels = statefulPod.Spec.ServiceTemplate.Selector
	}
	return &pod
}

func (p *PodController) CreatePod(ctx context.Context, pod *corev1.Pod) error {

	if err := p.Create(ctx, pod); err != nil {
		return err
	}
	return nil
}

func (p *PodController) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	if err := p.Delete(ctx, pod); err != nil {
		return err
	}
	return nil
}

func (p *PodController) DeletePodMandatory(ctx context.Context, pod *corev1.Pod, statefulPod *statefulpodv1.StatefulPod) error {
	if err := p.Delete(ctx, pod, client.DeleteOption(client.GracePeriodSeconds(0)), client.DeleteOption(client.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
		return err
	}
	if statefulPod.Spec.PvcTemplate != nil {
		pvcName := pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName
		if pvc, err, ok := pvccontrl.NewPvcController(p.Client).IsPvcExist(ctx, types.NamespacedName{
			Namespace: pod.Namespace,
			Name:      pvcName,
		}); err == nil && ok {
			return pvccontrl.NewPvcController(p.Client).DeletePVC(ctx, pvc)
		} else if err != nil {
			return err
		}
	}
	return nil
}

// 判断pod 是否应该是要删除状态
func (p *PodController) JudgmentPodDel(pod *corev1.Pod) bool {
	if !pod.DeletionTimestamp.IsZero() {
		return true
	}
	return false
}

func (p *PodController) DeletePodAll(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	for i, v := range statefulPod.Status.PodStatusMes {
		if pod, err, ok := p.IsPodExist(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PodName,
		}); err == nil && ok {
			if err := p.Delete(ctx, pod); err != nil {
				return err
			}
			pvcName := pvccontrl.NewPvcController(p.Client).SetPvcName(statefulPod, i)
			if pvc, err, ok := pvccontrl.NewPvcController(p.Client).IsPvcExist(ctx, types.NamespacedName{
				Namespace: statefulPod.Namespace,
				Name:      pvcName,
			}); err == nil && ok {
				if statefulPod.Spec.PVRecyclePolicy == corev1.PersistentVolumeReclaimRetain {
					pvName := pvc.Spec.VolumeName
					if err := pvcontrl.NewPVController(p.Client).SetPVRetain(ctx, &pvName); err != nil {
						return err
					}
					if err := pvccontrl.NewPvcController(p.Client).DeletePVC(ctx, pvc); err != nil {
						return err
					}
					for {
						if _, err, ok := pvccontrl.NewPvcController(p.Client).IsPvcExist(ctx, types.NamespacedName{
							Namespace: pvc.Namespace,
							Name:      pvc.Name,
						}); err == nil && !ok {
							break
						}
					}
					if err := pvcontrl.NewPVController(p.Client).SetVolumeAvailable(ctx, &pvName); err != nil {
						return err
					}
				} else {
					if err := pvccontrl.NewPvcController(p.Client).DeletePVC(ctx, pvc); err != nil {
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
