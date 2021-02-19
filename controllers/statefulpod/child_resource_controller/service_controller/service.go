package service_controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	servicehandle "iapetos/controllers/service"
)

type ServiceController struct {
	client.Client
}

type ServiceContrlIntf interface {
	CreateService(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) (bool, error)
	RemoveServiceFinalizer(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error
}

func NewServiceController(client client.Client) ServiceContrlIntf {
	return &ServiceController{client}
}

func (servicectl *ServiceController) CreateService(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) (bool, error) {
	servicehandler := servicehandle.NewServiceContrl(servicectl.Client)
	serviceName := servicehandler.SetServiceName(statefulPod)
	if service, err, ok := servicehandler.IsServiceExits(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      serviceName,
	}); err == nil && !ok { // service 不存在 ,创建 service
		serviceTemplate := servicehandler.ServiceTemplate(statefulPod)
		if err := servicehandler.CreateService(ctx, serviceTemplate); err != nil {
			return false, err
		}
	} else if err == nil && ok {
		// service创建成功，设置finalizer防止误删
		if err := servicehandler.SetFinalizer(ctx, service); err != nil {
			return false, err
		}
		return true, nil
	} else {
		return false, err
	}
	return false, nil
}

func (servicectl *ServiceController) RemoveServiceFinalizer(ctx context.Context, statefulPod *statefulpodv1.StatefulPod) error {
	servicehandler := servicehandle.NewServiceContrl(servicectl.Client)
	serviceName := servicehandler.SetServiceName(statefulPod)
	if service, err, ok := servicehandler.IsServiceExits(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      serviceName,
	}); err == nil && ok { // service 存在 ,清空 finalizer
		service.Finalizers = nil
		if err := servicectl.Update(ctx, service); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return nil
}
