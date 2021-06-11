package service

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)

type Service struct {
	*services.Resource
}

func NewPodService(client client.Client) services.ServiceInf {
	clientMsg := services.NewResource(client)
	clientMsg.Log.WithName("service")
	return &Service{clientMsg}
}

func (svc *Service) DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error {
	return nil
}

func (svc *Service) GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string {
	name := svc.SetServiceName(statefulPod)
	return &name
}

func (svc *Service) CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{} {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      svc.SetServiceName(statefulPod),
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				iapetosapiv1.GroupVersion.String(): "true",
				services.ParentNmae:                statefulPod.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    services.StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.ServiceTemplate.DeepCopy(),
	}
}

func (svc *Service) IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool) {
	var service corev1.Service
	if err := svc.Get(ctx, nameSpaceName, &service); err != nil {
		if client.IgnoreNotFound(err) != nil {
			svc.Log.Error(err, "get svc error")
		}
		return nil, false
	}
	return &service, true
}

func (svc *Service) IsResourceVersionSame(ctx context.Context, obj interface{}) bool {
	service := obj.(*corev1.Service)
	if newSvc, ok := svc.IsExists(ctx, types.NamespacedName{
		Namespace: service.Namespace,
		Name:      service.Name,
	}); !ok {
		return false
	} else {
		// 判断 resource version 是否一致
		newVersion := newSvc.(*corev1.Service).ResourceVersion
		if service.ResourceVersion != newVersion {
			return false
		} else {
			return true
		}
	}
}

func (svc *Service) Create(ctx context.Context, obj interface{}) (interface{}, error) {
	service := obj.(*corev1.Service)
	if err := svc.Client.Create(ctx, service); err != nil {
		svc.Log.Error(err, "create service error")
		return nil, err
	}
	return service, nil
}

func (svc *Service) Update(ctx context.Context, obj interface{}) (interface{}, error) {
	service := obj.(*corev1.Service)
	if svc.IsResourceVersionSame(ctx, service) {
		if err := svc.Client.Update(ctx, service); err != nil {
			svc.Log.Error(err, "update service error")
			return nil, err
		}
	} else {
		svc.Log.Error(errors.New(""), services.ResourceVersionUnSame)
		return nil, errors.New("")
	}
	return service, nil
}

func (svc *Service) Delete(ctx context.Context, obj interface{}) error {
	service := obj.(*corev1.Service)
	if err := svc.Client.Delete(ctx, service); err != nil {
		svc.Log.Error(err, "delete service error")
		return err
	}
	return nil
}
