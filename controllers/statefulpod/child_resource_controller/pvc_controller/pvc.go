package pvc_controller

import (
	"context"
	"fmt"

	statefulpodv1 "github.com/q8s-io/iapetos/api/v1"
	pvcservice "github.com/q8s-io/iapetos/services/pvc"
	"github.com/q8s-io/iapetos/tools"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type PvcController struct {
	client.Client
}

type PvcContrlIntf interface {
	ExpansionPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (*statefulpodv1.PvcStatus, error)
	ShrinkPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error)
	MonitorPVCStatus(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool
}

func NewPvcController(client client.Client) PvcContrlIntf {
	return &PvcController{client}
}

func (pvcctrl *PvcController) ExpansionPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (*statefulpodv1.PvcStatus, error) {
	pvcHandler := pvcservice.NewPvcService(pvcctrl.Client)
	pvcName := pvcHandler.SetPvcName(statefulPod, index)
	if _, err, ok := pvcHandler.IsPvcExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && !ok { // pvc 不存在。创建 pvc
		pvcTemplate, _ := pvcHandler.PvcTemplate(ctx, statefulPod, pvcName, index)
		if err := pvcHandler.CreatePVC(ctx, pvcTemplate); err != nil {
			return nil, err
		}
		fmt.Println("-----------create pvc successful")
		pvcStatus := &statefulpodv1.PvcStatus{
			Index:        tools.IntToIntr32(index),
			PvcName:      pvcName,
			Status:       corev1.ClaimPending,
			AccessModes:  statefulPod.Spec.PvcTemplate.AccessModes,
			StorageClass: *statefulPod.Spec.PvcTemplate.StorageClassName,
		}
		return pvcStatus, nil
	} else if err == nil && ok { // pvc 存在，pvcStatus不变
		pvcStatus := statefulPod.Status.PvcStatusMes[index]
		return &pvcStatus, nil
	} else {
		return nil, err
	}
}

func (pvcctrl *PvcController) ShrinkPvc(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, index int) (bool, error) {
	pvcHandler := pvcservice.NewPvcService(pvcctrl.Client)
	pvcName := pvcHandler.SetPvcName(statefulPod, index-1)
	if pvc, err, ok := pvcHandler.IsPvcExist(ctx, types.NamespacedName{
		Namespace: statefulPod.Namespace,
		Name:      pvcName,
	}); err == nil && ok { // pvc 存在，删除 pvc
		if !pvc.DeletionTimestamp.IsZero() { // pvc 正在删除
			return false, nil
		}
		if err := pvcHandler.DeletePVC(ctx, pvc); err != nil {
			return false, err
		}
	} else if err == nil && !ok {
		// pvc 删除成功
		return true, nil
	} else {
		return false, err
	}
	return false, nil
}

func (pvcctrl *PvcController) MonitorPVCStatus(ctx context.Context, statefulPod *statefulpodv1.StatefulPod, pvc *corev1.PersistentVolumeClaim, index int) bool {
	if index >= len(statefulPod.Status.PvcStatusMes) {
		return false
	}
	if !pvc.DeletionTimestamp.IsZero() {
		if statefulPod.Status.PvcStatusMes[index].Status == pvcservice.Deleting {
			return false
		}
		statefulPod.Status.PvcStatusMes[index].Status = pvcservice.Deleting
		return true
	}
	if pvc.Status.Phase == corev1.ClaimBound {
		if statefulPod.Status.PvcStatusMes[index].Status == corev1.ClaimBound {
			return false
		}
		statefulPod.Status.PvcStatusMes[index].Status = corev1.ClaimBound
		capicity := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		statefulPod.Status.PvcStatusMes[index].Capacity = capicity.String()
		return true
	}
	return false
}
