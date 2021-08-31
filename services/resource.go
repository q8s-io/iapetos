package services

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	resourcecfg "github.com/q8s-io/iapetos/initconfig"
)

type ServiceInf interface {
	Create(ctx context.Context, obj interface{}) (interface{}, error)
	Update(ctx context.Context, obj interface{}) (interface{}, error)
	Delete(ctx context.Context, obj interface{}) error
	IsExists(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, bool)
	CreateTemplate(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, name string, index int) interface{}
	GetName(statefulPod *iapetosapiv1.StatefulPod, index int) *string
	IsResourceVersionSame(ctx context.Context, obj interface{}) bool
	DeleteMandatory(ctx context.Context, obj interface{}, statefulPod *iapetosapiv1.StatefulPod) error
	Get(ctx context.Context, nameSpaceName types.NamespacedName) (interface{}, error)
}

const (
	ResourceVersionUnSame = "ResourceVersionUnSame"
	ParentNmae            = "parentName"
	StatefulPod           = "StatefulPod"
	Index                 = "index"
)

type Resource struct {
	client.Client
	Log logr.Logger
}

func NewResource(client client.Client) *Resource {
	return &Resource{client, ctrl.Log.WithName("service")}
}

func (r *Resource) IsPvCanUse(ctx context.Context, pvNameSpaceName *types.NamespacedName) (*string, bool) {
	var pv corev1.PersistentVolume
	if err := r.Get(ctx, *pvNameSpaceName, &pv); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "get pv error")
		}
		return nil, false
	}
	// pv 是否是 Available
	if pv.Status.Phase != corev1.VolumeAvailable {
		return nil, false
	}
	// 判断 pv所在的节点是否正常
	var nodeName string
	if name, ok := pv.Annotations["kubevirt.io/provisionOnNode"]; ok {
		nodeName = name
	} else {
		return nil, false
	}
	if !r.IsNodeReady(ctx, types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}) {
		return nil, false
	}
	return &nodeName, true
}

func (r *Resource) IsNodeReady(ctx context.Context, nodeName types.NamespacedName) bool {
	if nodeName.Name == "" {
		return true
	}
	var node corev1.Node
	// 判断 node 是否存在
	if err := r.Get(ctx, nodeName, &node); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "get node error")
		}
		return false
	}
	// 存在，判断是否超时，即判断 node.spec.conditions 最后一个元素的状态是否为 true，若不为 true，判断失联时间是否超时
	timeOut := time.Second * time.Duration(resourcecfg.StatefulPodResourceCfg.Node.Timeout)
	if node.Status.Conditions[len(node.Status.Conditions)-1].Status != corev1.ConditionTrue {
		lostConnectTime := node.Status.Conditions[len(node.Status.Conditions)-1].LastTransitionTime
		if time.Now().Sub(lostConnectTime.Time) >= timeOut {
			return false
		}
	}
	return true
}

func (r *Resource) SetPVCName(statefulPod *iapetosapiv1.StatefulPod, index int) string {
	if statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName == "" {
		statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName = "data"
	}
	return fmt.Sprintf("%v-%v-%v", statefulPod.Spec.PodTemplate.Volumes[0].PersistentVolumeClaim.ClaimName, statefulPod.Name, index)
}

func (r *Resource) SetServiceName(statefulPod *iapetosapiv1.StatefulPod) string {
	return fmt.Sprintf("%v-%v", statefulPod.Name, "service")
}
