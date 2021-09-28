package pvc_controller

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/api/v1"
	pvcservice "w.src.corp.qihoo.net/data-platform/infra/iapetos.git/services/pvc"
	"w.src.corp.qihoo.net/data-platform/infra/iapetos.git/tools"
)

type PVCCtrl struct {
	client.Client
}

const (
	Deleting      = corev1.PersistentVolumeClaimPhase("Deleting")
	CreateTimeOut = corev1.PodPhase("CreateTimeOut")
)

type PVCCtrlFunc interface {
	ExpansionPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PVCStatus, error)
	ShrinkPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
	MonitorPVCStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool
	DeletePvcAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool
	IsCreationPvcTimeout(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool
}

func NewPVCCtrl(client client.Client) PVCCtrlFunc {
	return &PVCCtrl{client}
}

func (pvcctrl *PVCCtrl) IsCreationPvcTimeout(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool {
	if statefulPod.Spec.PVCTemplate == nil {
		return true
	}
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	pvcName := pvcHandler.GetName(statefulPod, index)
	if obj, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *pvcName,
	}); ok {
		pvc := obj.(*corev1.PersistentVolumeClaim)
		// 删除pod
		_ = pvcHandler.Delete(ctx, pvc)
		return false
	} else {
		// 不存在
		return true
	}
}

func (pvcctrl *PVCCtrl) DeletePvcAll(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) bool {
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	sum := 0
	for _, v := range statefulPod.Status.PVCStatusMes {
		if pod, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
			Namespace: statefulPod.Namespace,
			Name:      v.PVCName,
		}); ok { // pod 存在，删除 pod
			fmt.Println("-------delete pvc--------")
			if err := pvcHandler.Delete(ctx, pod); err != nil {
				return false
			}
		} else {
			sum++
		}
	}
	if sum == len(statefulPod.Status.PVCStatusMes) {
		return true
	}
	return false
}

func (pvcctrl *PVCCtrl) ExpansionPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (*iapetosapiv1.PVCStatus, error) {
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	pvcName := pvcHandler.GetName(statefulPod, index)
	if _, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *pvcName,
	}); !ok { // pvc 不存在，创建 pvc
		pvcTemplate := pvcHandler.CreateTemplate(ctx, statefulPod, *pvcName, index)
		if _, err := pvcHandler.Create(ctx, pvcTemplate); err != nil {
			return nil, err
		}
		pvcStatus := &iapetosapiv1.PVCStatus{
			Index:        tools.IntToIntr32(index),
			PVCName:      *pvcName,
			Status:       corev1.ClaimPending,
			AccessModes:  statefulPod.Spec.PVCTemplate.AccessModes,
			StorageClass: *statefulPod.Spec.PVCTemplate.StorageClassName,
		}
		return pvcStatus, nil
		// pvc 存在，pvcStatus 不变
	} else {
		if index >= len(statefulPod.Status.PVCStatusMes) {
			return nil, errors.New("")
		}
		pvcStatus := statefulPod.Status.PVCStatusMes[index]
		return &pvcStatus, nil
	}
}

func (pvcctrl *PVCCtrl) ShrinkPVC(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) bool {
	pvcHandler := pvcservice.NewPVCService(pvcctrl.Client)
	pvcName := pvcHandler.GetName(statefulPod, index)
	if pvc, ok := pvcHandler.IsExists(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      *pvcName,
	}); ok { // pvc 存在，删除 pvc
		if err := pvcHandler.Delete(ctx, pvc); err != nil {
			return false
		}
		// pvc 删除成功
	} else {
		return true
	}
	return false
}

func (pvcctrl *PVCCtrl) MonitorPVCStatus(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool {
	if index >= len(statefulPod.Status.PVCStatusMes) {
		return false
	}
	if !pvc.DeletionTimestamp.IsZero() {
		if statefulPod.Status.PVCStatusMes[index].Status == Deleting || statefulPod.Status.PodStatusMes[index].Status == CreateTimeOut {
			return false
		}
		statefulPod.Status.PVCStatusMes[index].Status = Deleting
		return true
	}
	if pvc.Status.Phase == corev1.ClaimBound {
		if statefulPod.Status.PVCStatusMes[index].Status == corev1.ClaimBound {
			return false
		}
		statefulPod.Status.PVCStatusMes[index].Status = corev1.ClaimBound
		capicity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		statefulPod.Status.PVCStatusMes[index].Capacity = capicity.String()
		statefulPod.Status.PVCStatusMes[index].PVName = pvc.Spec.VolumeName
		return true
	}
	return false
}
