package pv

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PVController struct {
	client.Client
}

type PVContrlIntf interface {
	IsPVExists(ctx context.Context, namespaceName types.NamespacedName) (*corev1.PersistentVolume, error, bool)
	SetPVRetain(ctx context.Context, pvName *string) error
	SetVolumeAvailable(ctx context.Context, pvName *string) error
}

func NewPVController(client client.Client) PVContrlIntf {
	return &PVController{client}
}

// 判断 pv 是否存在
func (p *PVController) IsPVExists(ctx context.Context, namespaceName types.NamespacedName) (*corev1.PersistentVolume, error, bool) {
	var pv corev1.PersistentVolume
	if err := p.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}, &pv); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		return nil, err, false
	}
	return &pv, nil, true
}

// 将 pv 的删除策略设置为 Retain
func (p *PVController) SetPVRetain(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok { // pv 存在 将pv删除策略设置为 回收，同时将StorageClassName 设置为空
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		pv.Spec.StorageClassName = ""
		if err := p.updatePV(ctx, pv); err != nil { // 修改 pv状态，即设置 pv的删除策略为回收
			return err
		}
	}
	return nil
}

// 等待将 pv 的回收策略设置为 Retain
func (p *PVController) updatePV(ctx context.Context, pv *corev1.PersistentVolume) error {
	if err := p.Update(ctx, pv); err != nil {
		return err
	}
	// 循环等待，知道pv的删除策略成功设置为回收
	for {
		pv, _, _ = p.IsPVExists(ctx, types.NamespacedName{
			Namespace: "",
			Name:      pv.Name,
		})
		if pv == nil { // pv 不存在，退出
			break
		}
		// pv 删除策略成功设置为回收，退出
		if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
			break
		}
	}
	return nil
}

// 将 pv 状态设置为 Available
func (p *PVController) SetVolumeAvailable(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok { // pv存在，将pv设置为可用
		pv.Finalizers = nil
		pv.Spec.ClaimRef = nil
		pv.Status.Phase = corev1.VolumeAvailable
		if err := p.changePVStatus(ctx, pv); err != nil {
			return err
		}
	}
	return nil
}

// 修改 pv 状态 防止资源修改冲突
func (p *PVController) changePVStatus(ctx context.Context, pv *corev1.PersistentVolume) error {
	// 循环等待，知道 pv状态为可用
	for {
		if err := p.Update(ctx, pv, client.DryRunAll); err == nil {
			if err := p.Update(ctx, pv); err != nil {
				return err
			}
			return nil
		} else {
			// 更新失败，重新拉取最新状态，然后更新
			newPV, _, _ := p.IsPVExists(ctx, types.NamespacedName{
				Namespace: pv.Namespace,
				Name:      pv.Name,
			})
			if newPV == nil {
				break
			}
			newPV.Finalizers = pv.Finalizers
			newPV.Spec.ClaimRef = nil
			newPV.Status.Phase = corev1.VolumeAvailable
			pv = newPV
		}
	}
	return nil
}
