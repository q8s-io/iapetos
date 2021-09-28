package service_controller

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/api/v1"
	svcservice "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/services/service"
)

type ServiceController struct {
	client.Client
}

type ServiceContrlIntf interface {
	CreateService(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool
	//RemoveServiceFinalizer(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) error
}

func NewServiceController(client client.Client) ServiceContrlIntf {
	return &ServiceController{client}
}

func (servicectl *ServiceController) CreateService(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool {
	svcHandle := svcservice.NewPodService(servicectl.Client)
	serviceName := svcHandle.GetName(statefulPod, 0)
	if _, ok := svcHandle.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *serviceName,
	}); !ok {
		svcTemplate := svcHandle.CreateTemplate(ctx, statefulPod, "", 0)
		if _, err := svcHandle.Create(ctx, svcTemplate); err != nil {
			return false
		}
	} else {
		return true
	}
	return false
}
