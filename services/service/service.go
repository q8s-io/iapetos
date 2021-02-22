package service

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	statefulpodv1 "iapetos/api/v1"
	"iapetos/tools"
)

const (
	ParentNmae    = "parentName"
	StatefulPod   = "StatefulPod"
	FinalizerName = "kubernetes.io/service-protection"
)

type Service struct {
	client.Client
	Log logr.Logger
}

type ServiceIntf interface {
	SetServiceName(statefulPod *statefulpodv1.StatefulPod) string
	IsServiceExits(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Service, error, bool)
	ServiceTemplate(statefulPod *statefulpodv1.StatefulPod) *corev1.Service
	CreateService(ctx context.Context, service *corev1.Service) error
	SetFinalizer(ctx context.Context, service *corev1.Service) error
}

func NewServiceContrl(client client.Client) ServiceIntf {
	//serviceLog.WithName("service mesasge")
	return &Service{client, ctrl.Log.WithName("controllers").WithName("service")}
}

// 判断 service 是否存在
func (s *Service) IsServiceExits(ctx context.Context, namespaceName types.NamespacedName) (*corev1.Service, error, bool) {
	var service corev1.Service
	if err := s.Get(ctx, namespaceName, &service); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		s.Log.Error(err, "get service error")
		return nil, err, false
	} else {
		return &service, nil, true
	}
}

// 创建 service 模板
func (s *Service) ServiceTemplate(statefulPod *statefulpodv1.StatefulPod) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.SetServiceName(statefulPod),
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				statefulpodv1.GroupVersion.String(): "true",
				ParentNmae:                          statefulPod.Name,
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   statefulpodv1.GroupVersion.Group,
					Version: statefulpodv1.GroupVersion.Version,
					Kind:    StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.ServiceTemplate.DeepCopy(),
	}
}

// 创建 service
func (s *Service) CreateService(ctx context.Context, service *corev1.Service) error {
	if err := s.Create(ctx, service); err != nil {
		s.Log.Error(err, "create service error")
		return err
	}
	s.Log.V(1).Info("create service successfilly")
	return nil
}

// 设置 service name
func (s *Service) SetServiceName(statefulPod *statefulpodv1.StatefulPod) string {
	return fmt.Sprintf("%v-%v", statefulPod.Name, "service")
}

func (s *Service) SetFinalizer(ctx context.Context, service *corev1.Service) error {
	if !service.DeletionTimestamp.IsZero() {
		if !tools.ContainsString(service.Finalizers, FinalizerName) {
			service.Finalizers = append(service.Finalizers, FinalizerName)
			if err := s.Update(ctx, service); err != nil {
				s.Log.Error(err, "set finalizer error")
				return err
			}
		}
	}
	return nil
}
