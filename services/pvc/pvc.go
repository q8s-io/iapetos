package pvc

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "github.com/q8s-io/iapetos/api/v1"
	pvservice "github.com/q8s-io/iapetos/services/pv"
)

const (
	ParentNmae  = "parentName"
	StatefulPod = "StatefulPod"
	Index       = "index"
	Deleting    = "Deleting"
)

type PvcService struct {
	client.Client
	Log logr.Logger
}

type PvcServiceIntf interface {
	PvcTemplate(ctx context.Context, statefulpod *statefulpodv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error)
	CreatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error
	IsPvcExist(ctx context.Context, name types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool)
	DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error
	SetPvcName(statefulPod *statefulpodv1.StatefulPod, index int) string
}

func NewPvcService(client client.Client) PvcServiceIntf {
	//pvcLog.WithName("pvc message")
	return &PvcService{client, ctrl.Log.WithName("controllers").WithName("pvc")}
}

// 创建 pvc 模板
func (p *PvcService) PvcTemplate(ctx context.Context, statefulpod *statefulpodv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error) {
	pvHandler := pvservice.NewPVService(p.Client)
	if len(statefulpod.Spec.PVNames) != 0 && len(statefulpod.Spec.PVNames) > index {
		if _, ok := pvHandler.IsPVCanUse(ctx, statefulpod.Spec.PVNames[index]); ok {
			name := ""
			statefulpod.Spec.PvcTemplate.StorageClassName = &name
			statefulpod.Spec.PvcTemplate.VolumeName = statefulpod.Spec.PVNames[index]
		}
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcName,
			Namespace: statefulpod.Namespace,
			Annotations: map[string]string{
				statefulpodv1.GroupVersion.String(): "true",
				ParentNmae:                          statefulpod.Name,
				Index:                               strconv.Itoa(index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulpod, schema.GroupVersionKind{
					Group:   statefulpodv1.GroupVersion.Group,
					Version: statefulpodv1.GroupVersion.Version,
					Kind:    StatefulPod,
				}),
			},
		},
		Spec: *statefulpod.Spec.PvcTemplate.DeepCopy(),
	}, nil

}

// 判断 pvc 是否存在
func (p *PvcService) IsPvcExist(ctx context.Context, nameSpaceName types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool) {
	var pvc corev1.PersistentVolumeClaim
	if err := p.Get(ctx, types.NamespacedName{
		Namespace: nameSpaceName.Namespace,
		Name:      nameSpaceName.Name,
	}, &pvc); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		p.Log.Error(err, "get pvc error")
		return nil, err, false
	}
	return &pvc, nil, true
}

// 创建 pvc
func (p *PvcService) CreatePVC(ctx context.Context, createPvc *corev1.PersistentVolumeClaim) error {
	if err := p.Create(ctx, createPvc); err != nil {
		p.Log.Error(err, "create pvc error")
		return err
	}
	return nil
}

// 删除 pvc
func (p *PvcService) DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error {
	if err := p.Delete(ctx, deletePVC); err != nil {
		p.Log.Error(err, "delete pvc error")
		return err
	}
	return nil
}

// 设置 pvc name
func (p *PvcService) SetPvcName(statefulPod *statefulpodv1.StatefulPod, index int) string {
	if statefulPod.Spec.PvcTemplate == nil { // pvc 不需要创建，返回 none
		return "none"
	}
	return fmt.Sprintf("%v-%v-%v", statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName, statefulPod.Name, index)
}
