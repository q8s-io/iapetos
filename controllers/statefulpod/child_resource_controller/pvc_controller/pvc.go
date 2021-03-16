package pvc_controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	pvcservice "github.com/q8s-io/iapetos/services/pvc"
	"github.com/q8s-io/iapetos/tools"
)

type PVCCtrl struct {
	client.Client
}

type PVCCtrlFunc interface {
	ExpansionPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PVCStatus, error)
	ShrinkPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (bool, error)
	MonitorPVCStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool
}

func NewPVCCtrl(client client.Client) PVCCtrlFunc {
	return &PVCCtrl{client}
}

func (pvcctrl *PVCCtrl) ExpansionPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PVCStatus, error) {
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	pvcName := pvcHandler.SetPVCName(statefulPod, index)

	if _, err, ok := pvcHandler.IsPVCExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && !ok { // pvc 不存在，创建 pvc
		pvcTemplate, _ := pvcHandler.PVCTemplate(ctx, statefulPod, pvcName, index)
		if err := pvcHandler.CreatePVC(ctx, pvcTemplate); err != nil {
			return nil, err
		}
		pvcStatus := &iapetosapiv1.PVCStatus{
			Index:        tools.IntToIntr32(index),
			PVCName:      pvcName,
			Status:       corev1.ClaimPending,
			AccessModes:  statefulPod.Spec.PVCTemplate.AccessModes,
			StorageClass: *statefulPod.Spec.PVCTemplate.StorageClassName,
		}
		return pvcStatus, nil
		// pvc 存在，pvcStatus 不变
	} else if err == nil && ok {
		pvcStatus := statefulPod.Status.PVCStatusMes[index]
		return &pvcStatus, nil
	} else {
		return nil, err
	}
}

func (pvcctrl *PVCCtrl) ShrinkPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (bool, error) {
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	pvcName := pvcHandler.SetPVCName(statefulPod, index-1)
	if pvc, err, ok := pvcHandler.IsPVCExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && ok { // pvc 存在，删除 pvc
		// pvc 正在删除
		if !pvc.DeletionTimestamp.IsZero() {
			return false, nil
		}
		if err := pvcHandler.DeletePVC(ctx, pvc); err != nil {
			return false, err
		}
		// pvc 删除成功
	} else if err == nil && !ok {
		return true, nil
	} else {
		return false, err
	}
	return false, nil
}

func (pvcctrl *PVCCtrl) MonitorPVCStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool {
	if index >= len(statefulPod.Status.PVCStatusMes) {
		return false
	}
	if !pvc.DeletionTimestamp.IsZero() {
		if statefulPod.Status.PVCStatusMes[index].Status == pvcservice.Deleting {
			return false
		}
		statefulPod.Status.PVCStatusMes[index].Status = pvcservice.Deleting
		return true
	}
	if pvc.Status.Phase == corev1.ClaimBound {
		if statefulPod.Status.PVCStatusMes[index].Status == corev1.ClaimBound {
			return false
		}
		statefulPod.Status.PVCStatusMes[index].Status = corev1.ClaimBound
		capicity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		statefulPod.Status.PVCStatusMes[index].Capacity = capicity.String()
		return true
	}
	return false
}
