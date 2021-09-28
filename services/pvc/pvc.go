package pvc

import (
	"context"
	"errors"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/api/v1"
	"w.src.corp.qihoo.net/data-platform/infra/iapetos.git/services"
)

type PVCService struct {
	*services.Resource
}

func NewPVCService(client client.Client) services.ServiceInf {
	clientMsg := services.NewResource(client)
	clientMsg.Log.WithName("pvc")
	return &PVCService{clientMsg}
}

func (pvc *PVCService) DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error {
	pvcObj := obj.(*corev1.PersistentVolumeClaim)
	if err := pvc.Client.Delete(ctx, pvcObj, client.DeleteOption(client.GracePeriodSeconds(0)), client.DeleteOption(client.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
		pvc.Log.Error(err, "delete pvc mandatory error")
		return err
	}
	return nil
}

func (pvc *PVCService) GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string {
	name := pvc.SetPVCName(statefulPod, index)
	return &name
}

func (pvc *PVCService) CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{} {

	if len(statefulPod.Spec.PVNames) != 0 && len(statefulPod.Spec.PVNames) > index {
		if _, ok := pvc.IsPvCanUse(ctx, &types.NamespacedName{
			Namespace: "",
			Name:      statefulPod.Spec.PVNames[index],
		}); ok {
			name := ""
			statefulPod.Spec.PVCTemplate.StorageClassName = &name
			statefulPod.Spec.PVCTemplate.VolumeName = statefulPod.Spec.PVNames[index]
		}
	}
	return &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				iapetosapiv1.GroupVersion.String(): "true",
				services.ParentNmae:                statefulPod.Name,
				services.Index:                     strconv.Itoa(index),
			},
			Labels: map[string]string{
				services.ParentNmae: statefulPod.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    services.StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.PVCTemplate.DeepCopy(),
	}
}

func (pvc *PVCService) IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool) {
	var pvcObj corev1.PersistentVolumeClaim
	if err := pvc.Client.Get(ctx, nameSpaceName, &pvcObj); err != nil {
		if client.IgnoreNotFound(err) != nil {
			pvc.Log.Error(err, "get pvc error")
		}
		return nil, false
	}
	return &pvcObj, true
}

func (pvc *PVCService) IsResourceVersionSame(ctx context.Context, obj interface{}) bool {
	pvcObj := obj.(*corev1.PersistentVolumeClaim)
	if newPvc, ok := pvc.IsExists(ctx, types.NamespacedName{
		Namespace: pvcObj.Namespace,
		Name:      pvcObj.Name,
	}); !ok {
		return false
	} else {
		// 判断 resource version 是否一致
		newVersion := newPvc.(*corev1.PersistentVolumeClaim).ResourceVersion
		if pvcObj.ResourceVersion != newVersion {
			return false
		} else {
			return true
		}
	}
}

func (pvc *PVCService) Create(ctx context.Context, obj interface{}) (interface{}, error) {
	pvcObj := obj.(*corev1.PersistentVolumeClaim)
	if err := pvc.Client.Create(ctx, pvcObj); err != nil {
		pvc.Log.Error(err, "create pvc error")
		return nil, err
	}
	return pvcObj, nil
}

func (pvc *PVCService) Update(ctx context.Context, obj interface{}) (interface{}, error) {
	pvcObj := obj.(*corev1.PersistentVolumeClaim)
	if pvc.IsResourceVersionSame(ctx, pvcObj) {
		if err := pvc.Client.Update(ctx, pvcObj); err != nil {
			pvc.Log.Error(err, "update pvc error")
			return nil, err
		}
	} else {
		pvc.Log.Error(errors.New(""), services.ResourceVersionUnSame)
		return nil, errors.New("")
	}
	return pvcObj, nil
}

func (pvc *PVCService) Delete(ctx context.Context, obj interface{}) error {
	pvcObj := obj.(*corev1.PersistentVolumeClaim)
	if err := pvc.Client.Delete(ctx, pvcObj); err != nil && client.IgnoreNotFound(err) != nil {
		pvc.Log.Error(err, "delete pvc error")
		return err
	}
	return nil
}

func (pvc *PVCService) Get(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, error) {
	return nil, nil
}
