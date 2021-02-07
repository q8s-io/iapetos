package node

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	resourcecfg "iapetos/initconfig"
)

type NodeController struct {
	client.Client
}

type NodeContrlIntf interface {
	IsNodeReady(ctx context.Context, nodeName string) bool
}

func NewNodeContrl(client client.Client) NodeContrlIntf {
	return &NodeController{client}
}

// 判断 node 是否正常
func (n *NodeController) IsNodeReady(ctx context.Context, nodeName string) bool {
	if nodeName == "" {
		return true
	}
	var node corev1.Node
	if err := n.Get(ctx, types.NamespacedName{
		Namespace: "",
		Name:      nodeName,
	}, &node); err != nil {
		return false
	} // 判断 node是否存在
	// 存在，判断是否超时 即判断node.spec.conditions最后一个元素的状态是否为true ,若不为true，
	// 判断失联时间是否超时
	timeOut := time.Second * time.Duration(resourcecfg.StatefulPodResourceCfg.Node.Timeout)
	if node.Status.Conditions[len(node.Status.Conditions)-1].Status != corev1.ConditionTrue {
		lostConactTime := node.Status.Conditions[len(node.Status.Conditions)-1].LastTransitionTime
		if time.Now().Sub(lostConactTime.Time) >= timeOut {
			return false
		}
	}
	return true
}
