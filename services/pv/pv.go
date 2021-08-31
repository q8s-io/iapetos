package pv

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)

type PVService struct {
	*services.Resource
}

func NewPVService(client client.Client) services.ServiceInf {
	clientMsg := services.NewResource(client)
	clientMsg.Log.WithName("pv")
	return &PVService{clientMsg}
}

func (pv *PVService) GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string {
	return &statefulPod.Status.PVCStatusMes[index].PVName
}

func (pv *PVService) DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error {
	return nil
}

func (pv *PVService) CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{} {
	return nil
}

func (pv *PVService) IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool) {
	var pvObj corev1.PersistentVolume
	if err := pv.Client.Get(ctx, nameSpaceName, &pvObj); err != nil {
		if client.IgnoreNotFound(err) != nil {
			pv.Log.Error(err, "get pv error")
		}
		return nil, false
	}
	return &pvObj, true

}

func (pv *PVService) IsResourceVersionSame(ctx context.Context, obj interface{}) bool {
	pvObj := obj.(*corev1.PersistentVolume)
	if newPv, ok := pv.IsExists(ctx, types.NamespacedName{
		Namespace: pvObj.Namespace,
		Name:      pvObj.Name,
	}); !ok {
		return false
	} else {
		// 判断 resource version 是否一致
		newVersion := newPv.(*corev1.PersistentVolume).ResourceVersion
		if pvObj.ResourceVersion != newVersion {
			return false
		} else {
			return true
		}
	}
}

func (pv *PVService) Create(ctx context.Context, obj interface{}) (interface{}, error) {
	return nil, nil
}

func (pv *PVService) Update(ctx context.Context, obj interface{}) (interface{}, error) {
	pvObj := obj.(*corev1.PersistentVolume)
	if pv.IsResourceVersionSame(ctx, pvObj) {
		if err := pv.Client.Update(ctx, pvObj); err != nil {
			pv.Log.Error(err, "update pv error")
			return nil, err
		}
	} else {
		pv.Log.Error(errors.New(""), services.ResourceVersionUnSame)
		return nil, errors.New("")
	}
	return pvObj, nil
}

func (pv *PVService) Delete(ctx context.Context, obj interface{}) error {
	return nil
}

func (pv *PVService) Get(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, error) {
	var pvObj corev1.PersistentVolume
	if err := pv.Client.Get(ctx, nameSpaceName, &pvObj); err != nil {
		return nil, err
	}
	return &pvObj, nil
}
