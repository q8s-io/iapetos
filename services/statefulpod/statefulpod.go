package statefulpod

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)

type StatefulPodService struct {
	*services.Resource
}

func NewStatefulPod(client client.Client) services.ServiceInf {
	clientMsg := services.NewResource(client)
	clientMsg.Log.WithName("statefulPod")
	return &StatefulPodService{clientMsg}
}

func (sfp *StatefulPodService) DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error {
	return nil
}

func (sfp *StatefulPodService) CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{} {

	return nil
}

func (sfp *StatefulPodService) GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string {
	name := statefulPod.Name
	return &name
}

func (sfp *StatefulPodService) IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool) {
	var statefulPod iapetosapiv1.StatefulPod
	if err := sfp.Get(ctx, nameSpaceName, &statefulPod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			sfp.Log.Error(err, "get statefulPod error")
		}
		return nil, false
	}
	return &statefulPod, true
}

func (sfp *StatefulPodService) IsResourceVersionSame(ctx context.Context, obj interface{}) bool {
	statefulPod := obj.(*iapetosapiv1.StatefulPod)
	if newStatefulPod, ok := sfp.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      statefulPod.Name,
	}); !ok {
		return false
	} else {
		// 判断 resource version 是否一致
		newVersion := newStatefulPod.(*iapetosapiv1.StatefulPod).ResourceVersion
		if statefulPod.ResourceVersion != newVersion {
			return false
		} else {
			return true
		}
	}
}

func (sfp *StatefulPodService) Create(ctx context.Context, obj interface{}) (interface{}, error) {
	statefulPod := obj.(*iapetosapiv1.StatefulPod)
	if err := sfp.Client.Create(ctx, statefulPod); err != nil {
		sfp.Log.Error(err, "create statefulPod error")
		return nil, err
	}
	return statefulPod, nil
}

func (sfp *StatefulPodService) Update(ctx context.Context, obj interface{}) (interface{}, error) {
	statefulPod := obj.(*iapetosapiv1.StatefulPod)
	if sfp.IsResourceVersionSame(ctx, statefulPod) {
		if err := sfp.Client.Update(ctx, statefulPod); err != nil {
			sfp.Log.Error(err, "update statefulPod error")
			return nil, err
		}
	} else {
		sfp.Log.Error(errors.New(""), services.ResourceVersionUnSame)
		return nil, errors.New("")
	}
	return statefulPod, nil
}

func (sfp *StatefulPodService) Delete(ctx context.Context, obj interface{}) error {
	statefulPod := obj.(*iapetosapiv1.StatefulPod)
	if err := sfp.Client.Delete(ctx, statefulPod); err != nil {
		sfp.Log.Error(err, "delete statefulPod error")
		return err
	}
	return nil
}
