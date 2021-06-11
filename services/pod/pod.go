package pod

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	"github.com/q8s-io/iapetos/services"
)

const (
	nodeUnhealthy = "nodeUnhealthy"
	cockrochDB    = "cockrochDB"
)

type PodService struct {
	*services.Resource
}

func NewPodService(client client.Client) services.ServiceInf {
	clientMsg := services.NewResource(client)
	clientMsg.Log.WithName("pod")
	return &PodService{clientMsg}
}

func (p *PodService) CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{} {
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: statefulPod.Namespace,
			Annotations: map[string]string{
				iapetosapiv1.GroupVersion.String(): "true",
				services.ParentNmae:                statefulPod.Name,
				services.Index:                     fmt.Sprintf("%v", index),
			},
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(statefulPod, schema.GroupVersionKind{
					Group:   iapetosapiv1.GroupVersion.Group,
					Version: iapetosapiv1.GroupVersion.Version,
					Kind:    services.StatefulPod,
				}),
			},
		},
		Spec: *statefulPod.Spec.PodTemplate.DeepCopy(),
	}
	// 添加 hostname subdomain 用于 dns 发现
	pod.Spec.Hostname = name
	// 添加annotation
	p.addAnnotations(statefulPod, &pod)
	// 设置pvc
	p.setPvc(statefulPod, &pod, index)
	// 设置 labels
	p.setLabels(statefulPod, &pod)
	return &pod
}

func (p *PodService) GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string {
	name := fmt.Sprintf("%v-%v", statefulPod.Name, index)
	return &name
}
func (p *PodService) IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool) {
	var pod corev1.Pod
	if err := p.Get(ctx, nameSpaceName, &pod); err != nil {
		if client.IgnoreNotFound(err) != nil {
			p.Log.Error(err, "get pod error")
		}
		return nil, false
	}
	return &pod, true
}

func (p *PodService) IsResourceVersionSame(ctx context.Context, obj interface{}) bool {
	pod := obj.(*corev1.Pod)
	if newPod, ok := p.IsExists(ctx, types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Name,
	}); !ok {
		return false
	} else {
		// 判断 resource version 是否一致
		newVersion := newPod.(*corev1.Pod).ResourceVersion
		if pod.ResourceVersion != newVersion {
			return false
		} else {
			return true
		}
	}
}

func (p *PodService) Create(ctx context.Context, obj interface{}) (interface{}, error) {
	pod := obj.(*corev1.Pod)
	if err := p.Client.Create(ctx, pod); err != nil {
		p.Log.Error(err, "create pod error")
		return nil, err
	}
	return pod, nil
}

func (p *PodService) Update(ctx context.Context, obj interface{}) (interface{}, error) {
	pod := obj.(*corev1.Pod)
	if p.IsResourceVersionSame(ctx, pod) {
		if err := p.Client.Update(ctx, pod); err != nil {
			p.Log.Error(err, "update pod error")
			return nil, err
		}
	} else {
		p.Log.Error(errors.New(""), services.ResourceVersionUnSame)
		return nil, errors.New("")
	}
	return pod, nil
}

func (p *PodService) Delete(ctx context.Context, obj interface{}) error {
	pod := obj.(*corev1.Pod)
	if err := p.Client.Delete(ctx, pod); err != nil {
		p.Log.Error(err, "delete pod error")
		return err
	}
	return nil
}

func (p *PodService) DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error {
	pod := obj.(*corev1.Pod)
	// 用于webhook校验
	if err := p.webhook(ctx, pod, statefulPod); err != nil {
		return err
	}
	if err := p.Client.Delete(ctx, pod, client.DeleteOption(client.GracePeriodSeconds(0)), client.DeleteOption(client.PropagationPolicy(metav1.DeletePropagationBackground))); err != nil {
		p.Log.Error(err, "delete pod mandatory error")
		return err
	}
	return nil
}

// 添加annotation 用于扩展
func (p *PodService) addAnnotations(statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod) {
	if _, ok := statefulPod.Annotations[cockrochDB]; ok {
		pod.Annotations[cockrochDB] = "true"
	}
}

// 用于webhook校验
func (p *PodService) webhook(ctx context.Context, pod *corev1.Pod, statefulPod *iapetosapiv1.StatefulPod) error {
	// 不包含codb返回
	if _, ok := pod.Annotations[cockrochDB]; !ok {
		return nil
	}
	// 包含nodeUnhealthy 返回，防止未添加上
	if _, ok := pod.Annotations["nodeUnhealthy"]; !ok {
		pod.Annotations["nodeUnhealthy"] = "true"
		_, _ = p.Update(ctx, pod)
		return errors.New("")
	}
	return nil
}

// TODO 判断 pvc 是否需要创建，只支持挂载一个 pvc
func (p *PodService) setPvc(statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod, index int) {
	if statefulPod.Spec.PVCTemplate != nil {
		pod.Spec.Volumes[0].PersistentVolumeClaim.ClaimName = p.SetPVCName(statefulPod, index)
	}
}

// 添加label 到 pod ，并添加subdomain
func (p *PodService) setLabels(statefulPod *iapetosapiv1.StatefulPod, pod *corev1.Pod) {
	lables := map[string]string{}
	if statefulPod.Spec.ServiceTemplate != nil {
		pod.Spec.Subdomain = p.SetServiceName(statefulPod)
		for k, v := range statefulPod.Spec.ServiceTemplate.Selector {
			if _, ok := lables[k]; !ok {
				lables[k] = v
			}
		}
	}
	if statefulPod.Spec.Selector != nil {
		for k, v := range statefulPod.Spec.Selector.MatchLabels {
			if _, ok := lables[k]; !ok {
				lables[k] = v
			}
		}
	}
	pod.Labels = lables
}
