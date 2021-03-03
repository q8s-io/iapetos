package node

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcecfg "github.com/q8s-io/iapetos/initconfig"
)

type NodeService struct {
	client.Client
	Log logr.Logger
}

type NodeServiceFunc interface {
	IsNodeReady(ctx context.Context, nodeName string) bool
}

func NewNodeService(client client.Client) NodeServiceFunc {
	return &NodeService{client, ctrl.Log.WithName("controllers").WithName("node")}
}

// 判断 node 是否正常
func (n *NodeService) IsNodeReady(ctx context.Context, nodeName string) bool {
	if nodeName == "" {
		return true
	}
	var node corev1.Node
	// 判断 node 是否存在
	if err := n.Get(ctx, types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}, &node); err != nil {
		n.Log.Error(err, "get node error")
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
