package pv_controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	pvservice "github.com/q8s-io/iapetos/services/pv"
)

type PVCtrl struct {
	client.Client
}

type PVCtrlFunc interface {
	SetPVRetain(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(bool)
	SetPVAvailable(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)bool
	//CodbPodReady(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(error)
}

func NewPodCtrl(client client.Client) PVCtrlFunc {
	return &PVCtrl{client}
}

func (pvctrl *PVCtrl)SetPVRetain(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(bool){
	if statefulPod.Spec.PVRecyclePolicy!=corev1.PersistentVolumeReclaimRetain{
		return true
	}
	sum:=0
	pvHandle:=pvservice.NewPVService(pvctrl.Client)
	for _,pvcStatus:=range statefulPod.Status.PVCStatusMes{
			if obj,ok:=pvHandle.IsExists(ctx,types.NamespacedName{
				Namespace: corev1.NamespaceAll,
				Name:    pvcStatus.PVName  ,
			});ok{
				pv:=obj.(*corev1.PersistentVolume)
				if pv.Spec.PersistentVolumeReclaimPolicy == corev1.PersistentVolumeReclaimRetain{
					sum++
					continue
				}
				pv.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
				pv.Spec.StorageClassName = ""
				if _,err:=pvHandle.Update(ctx,pv);err!=nil{
					return false
				}
		}
	}
	if sum==len(statefulPod.Status.PVCStatusMes){
		return true
	}
	return false
}


func (pvctrl *PVCtrl)SetPVAvailable(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)bool{
	if statefulPod.Spec.PVRecyclePolicy!=corev1.PersistentVolumeReclaimRetain{
		return true
	}
	sum:=0
	pvHandle:=pvservice.NewPVService(pvctrl.Client)
	for _,pvcStatus:=range statefulPod.Status.PVCStatusMes{
			if obj,ok:=pvHandle.IsExists(ctx,types.NamespacedName{
				Namespace: corev1.NamespaceAll,
				Name:      pvcStatus.PVName,
			});ok{
				pv:=obj.(*corev1.PersistentVolume)
				if pv.Status.Phase==corev1.VolumeAvailable{
					sum++
					continue
				}
				pv.Finalizers = nil
				pv.Spec.ClaimRef = nil
				pv.Status.Phase = corev1.VolumeAvailable
				if _,err:=pvHandle.Update(ctx,pv);err!=nil{
					return false
				}
			}
		}
	if sum==len(statefulPod.Status.PVCStatusMes){
		return true
	}
	return false
}