package pv

import (
	"context"
	"fmt"

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

func (p *PVController) IsPVExists(ctx context.Context, namespaceName types.NamespacedName) (*corev1.PersistentVolume, error, bool) {
	var pv corev1.PersistentVolume
	if err := p.Get(ctx, types.NamespacedName{
		Namespace: namespaceName.Namespace,
		Name:      namespaceName.Name,
	}, &pv); err != nil {
		fmt.Println("find PV error: ", err.Error())
		if client.IgnoreNotFound(err) == nil {
			return nil, nil, false
		}
		return nil, err, false
	}
	return &pv, nil, true
}

func (p *PVController) SetPVRetain(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok {
		pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		pv.Spec.StorageClassName = ""
		if err := p.updatePV(ctx, pv); err != nil {
			return err
		}
	}
	return nil
}

func (p *PVController) updatePV(ctx context.Context, pv *corev1.PersistentVolume) error {
	if err := p.Update(ctx, pv); err != nil {
		return err
	}
	for {
		pv, _, _ = p.IsPVExists(ctx, types.NamespacedName{
			Namespace: "",
			Name:      pv.Name,
		})
		if pv == nil {
			break
		}
		if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain {
			break
		}
	}
	return nil
}

func (p *PVController) SetVolumeAvailable(ctx context.Context, pvName *string) error {
	if pv, err, ok := p.IsPVExists(ctx, types.NamespacedName{
		Namespace: corev1.NamespaceAll,
		Name:      *pvName,
	}); err == nil && ok {
		pv.Finalizers = nil
		pv.Spec.ClaimRef = nil
		pv.Status.Phase = corev1.VolumeAvailable
		if err := p.changePVStatus(ctx, pv); err != nil {
			return err
		}
	}
	return nil
}

func (p *PVController) changePVStatus(ctx context.Context, pv *corev1.PersistentVolume) error {
	for {
		if err := p.Update(ctx, pv, client.DryRunAll); err == nil {
			if err := p.Update(ctx, pv); err != nil {
				return err
			}
			return nil
		} else {
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
