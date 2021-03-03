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

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	pvservice "github.com/q8s-io/iapetos/services/pv"
)

const (
	ParentNmae  = "parentName"
	StatefulPod = "StatefulPod"
	Index       = "index"
	Deleting    = "Deleting"
)

type PVCService struct {
	client.Client
	Log logr.Logger
}

type PVCServiceFunc interface {
	PVCTemplate(ctx context.Context, statefulpod *iapetosapiv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error)
	CreatePVC(ctx context.Context, pvc *corev1.PersistentVolumeClaim) error
	IsPVCExist(ctx context.Context, name types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool)
	DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error
	SetPVCName(statefulPod *iapetosapiv1.StatefulPod, index int) string
}

func NewPVCService(client client.Client) PVCServiceFunc {
	return &PVCService{client, ctrl.Log.WithName("controllers").WithName("pvc")}
}

// 创建 pvc 模板
func (p *PVCService) PVCTemplate(ctx context.Context, statefulpod *iapetosapiv1.StatefulPod, pvcName string, index int) (*corev1.PersistentVolumeClaim, error) {
	pvHandler := pvservice.NewPVService(p.Client)
	if len(statefulpod.Spec.PVNames) != 0 && len(statefulpod.Spec.PVNames) > index {
		if _, ok := pvHandler.IsPVCanUse(ctx, statefulpod.Spec.PVNames[index]); ok {
			name := ""
			statefulpod.Spec.PVCTemplate.StorageClassName = &name
			statefulpod.Spec.PVCTemplate.VolumeName = statefulpod.Spec.PVNames[index]
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
				iapetosapiv1.GroupVersion.String(): "true",
				ParentNmae:                          statefulpod.Name,
				Index:                               strconv.Itoa(index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulpod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    StatefulPod,
				}),
			},
		},
		Spec: *statefulpod.Spec.PVCTemplate.DeepCopy(),
	}, nil

}

// 判断 pvc 是否存在
func (p *PVCService) IsPVCExist(ctx context.Context, nameSpaceName types.NamespacedName) (*corev1.PersistentVolumeClaim, error, bool) {
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
func (p *PVCService) CreatePVC(ctx context.Context, createPvc *corev1.PersistentVolumeClaim) error {
	if err := p.Create(ctx, createPvc); err != nil {
		p.Log.Error(err, "create pvc error")
		return err
	}
	return nil
}

// 删除 pvc
func (p *PVCService) DeletePVC(ctx context.Context, deletePVC *corev1.PersistentVolumeClaim) error {
	if err := p.Delete(ctx, deletePVC); err != nil {
		p.Log.Error(err, "delete pvc error")
		return err
	}
	return nil
}

// 设置 pvc name
func (p *PVCService) SetPVCName(statefulPod *iapetosapiv1.StatefulPod, index int) string {
	if statefulPod.Spec.PVCTemplate == nil { // pvc 不需要创建，返回 none
		return "none"
	}
	if statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName == "" {
		statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName = "data"
	}
	return fmt.Sprintf("%v-%v-%v", statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName, statefulPod.Name, index)
}
