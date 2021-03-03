package service_controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	svcservice "github.com/q8s-io/iapetos/services/service"
)

type ServiceController struct {
	client.Client
}

type ServiceContrlIntf interface {
	CreateService(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) (bool, error)
	RemoveServiceFinalizer(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error
}

func NewServiceController(client client.Client) ServiceContrlIntf {
	return &ServiceController{client}
}

func (servicectl *ServiceController) CreateService(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) (bool, error) {
	serviceable := svcservice.NewServiceContrl(servicectl.Client)
	serviceName := serviceable.SetServiceName(statefulPod)
	if service, err, ok := serviceable.IsServiceExits(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      serviceName,
	}); err == nil && !ok { // service 不存在，创建 service
		serviceTemplate := serviceable.ServiceTemplate(statefulPod)
		if err := serviceable.CreateService(ctx, serviceTemplate); err != nil {
			return false, err
		}
	} else if err == nil && ok {
		// service 创建成功，设置 finalizer 防止误删
		if err := serviceable.SetFinalizer(ctx, service); err != nil {
			return false, err
		}
		return true, nil
	} else {
		return false, err
	}
	return false, nil
}

func (servicectl *ServiceController) RemoveServiceFinalizer(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error {
	serviceable := svcservice.NewServiceContrl(servicectl.Client)
	serviceName := serviceable.SetServiceName(statefulPod)
	if service, err, ok := serviceable.IsServiceExits(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      serviceName,
	}); err == nil && ok { // service 存在，清空 finalizer
		service.Finalizers = nil
		if err := servicectl.Update(ctx, service); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
