package statefulpod

import (
	"context"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	iapetosapiv1 "github.com/q8s-io/iapetos/api/v1"
	podctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pod_controller"
	pvctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pv_controller"
	pvcctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/pvc_controller"
	svcctrl "github.com/q8s-io/iapetos/controllers/statefulpod/child_resource_controller/service_controller"
	"github.com/q8s-io/iapetos/services/statefulpod"
	"github.com/q8s-io/iapetos/tools"
)

const (
	ParentNmae = "parentName"
	WaitTime = time.Duration(time.Second*2)
)

type StatefulPodCtrl struct {
	client.Client
	sync.RWMutex
}

type StatefulPodCtrlFunc interface {
	CoreCtrl(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) (ctrl.Result,error)
	MonitorPodStatus(ctx context.Context, pod *corev1.Pod) (ctrl.Result,error)
	MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (ctrl.Result,error)
}

func NewStatefulPodCtrl(client client.Client) StatefulPodCtrlFunc {
	return &StatefulPodCtrl{client, sync.RWMutex{}}
}

// StatefulPod 控制器
// len(statefulPod.Status.PodStatusMes) < int(*statefulPod.Spec.Size) 扩容
// len(statefulPod.Status.PodStatusMes) > int(*statefulPod.Spec.Size) 缩容
// len(statefulPod.Status.PodStatusMes) == int(*statefulPod.Spec.Size) 设置 Finalizer，维护
func (s *StatefulPodCtrl) CoreCtrl(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod)(ctrl.Result,error)  {
	if !statefulPod.DeletionTimestamp.IsZero(){
		return s.deleteStatefulPod(ctx,statefulPod)
	}
	lenStatus := len(statefulPod.Status.PodStatusMes)
	lenSpec:=int(*statefulPod.Spec.Size)
	if lenStatus != 0 && (statefulPod.Status.PodStatusMes[lenStatus-1].Status == podctrl.Preparing || statefulPod.Status.PVCStatusMes[lenStatus-1].Status == corev1.ClaimPending) {
		lenStatus--
	}
	if lenStatus < lenSpec {
		return s.expansion(ctx, statefulPod, lenStatus)
	} else if lenStatus > lenSpec {
		return s.shrink(ctx, statefulPod, lenStatus)
	} else {
		if result,err := s.setFinalizer(ctx, statefulPod); err != nil {
			return result,nil
		}
		return s.maintain(ctx, statefulPod)
	}
}

func (s *StatefulPodCtrl)deleteStatefulPod(ctx context.Context,statefulPod *iapetosapiv1.StatefulPod)(ctrl.Result,error){
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	pvCtrl:=pvctrl.NewPodCtrl(s.Client)
		myFinalizerName := iapetosapiv1.GroupVersion.String()
		// 删除 statefulPod
		if tools.MatchStringFromArray(statefulPod.Finalizers, myFinalizerName) {
			// 删除 pod、pvc
			// 设置所有pv为Retain
			if !pvCtrl.SetPVRetain(ctx,statefulPod) {
				return ctrl.Result{RequeueAfter: WaitTime},nil
			}
			// 删除所有pod pvc，
			if !podctrl.NewPodCtrl(s.Client).DeletePodAll(ctx, statefulPod) {
				return ctrl.Result{RequeueAfter: WaitTime},nil
			}
			// 将所有pv置为Available
			if !pvCtrl.SetPVAvailable(ctx,statefulPod){
				return ctrl.Result{RequeueAfter: WaitTime}, nil
			}
			// 移除 service 的 finalizer
			/*if err := svcctrl.NewServiceController(s.Client).RemoveServiceFinalizer(ctx, statefulPod); err != nil {
				return err
			}*/
			statefulPod.Finalizers = tools.RemoveString(statefulPod.Finalizers, myFinalizerName)
			if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil{
				return ctrl.Result{
					RequeueAfter: WaitTime,
				},nil
			}
		}
	return ctrl.Result{},nil
}

// 扩容
// 创建 service
// pvc 需要创建，则创建 pvc，不需要 pvcStatus 设置为 0 值
// index == len(statefulPod.Status.PodStatusMes) 代表创建
// index != len(statefulPod.Status.PodStatusMes) 代表维护
func (s *StatefulPodCtrl) expansion(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (ctrl.Result,error) {
	defer func() {
		if recover()!=nil{
			time.Sleep(time.Second)
		}
	}()
	var podStatus *iapetosapiv1.PodStatus
	var pvcStatus *iapetosapiv1.PVCStatus
	var err error
	serviceCtrl := svcctrl.NewServiceController(s.Client)
	podCtrl := podctrl.NewPodCtrl(s.Client)
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	// 索引为 0，且需要生成 service
	if index == 0 && statefulPod.Spec.ServiceTemplate != nil {
		if ok:= serviceCtrl.CreateService(ctx, statefulPod);!ok{
			// 若创建，则等待5秒
			return ctrl.Result{
				RequeueAfter: WaitTime,
			},nil
		}
	}
	if podStatus, err = podCtrl.ExpansionPod(ctx, statefulPod, index); err != nil {
		return ctrl.Result{Requeue: true},err
	}
	// 判断pod是否创建超时，若超时删除pod，pvc
	if !podCtrl.IsCreationTimeout(ctx, statefulPod, index) {
		if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil{
			return ctrl.Result{
				RequeueAfter: WaitTime,
			},nil
		}
		return ctrl.Result{RequeueAfter: time.Second*2},nil
	}
	if statefulPod.Spec.PVCTemplate != nil {
		if pvcStatus, err = pvcCtrl.ExpansionPVC(ctx, statefulPod, index); err != nil {
			return ctrl.Result{},err
		}
	} else {
		pvcStatus = &iapetosapiv1.PVCStatus{
			Index:       tools.IntToIntr32(index),
			PVCName:     "none",
			Status:      "",
			AccessModes: []corev1.PersistentVolumeAccessMode{"none"},
		}
	}
	// 等于index代表是第一次扩容，不等代表维护
	if len(statefulPod.Status.PodStatusMes) == index {
		statefulPod.Status.PodStatusMes = append(statefulPod.Status.PodStatusMes, *podStatus)
		statefulPod.Status.PVCStatusMes = append(statefulPod.Status.PVCStatusMes, *pvcStatus)
	} else {
		statefulPod.Status.PodStatusMes[index] = *podStatus
		statefulPod.Status.PVCStatusMes[index] = *pvcStatus
	}
	if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil{
		return ctrl.Result{
			RequeueAfter: WaitTime,
		},nil
	}
	return ctrl.Result{},nil
}

// 缩容
// 若 pvc 存在，删除 pvc
func (s *StatefulPodCtrl) shrink(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod, index int) (ctrl.Result,error) {
	podCtrl := podctrl.NewPodCtrl(s.Client)
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	// 判断 pod 是否删除完毕
	if ok := podCtrl.ShrinkPod(ctx, statefulPod, index-1); !ok {
		return ctrl.Result{RequeueAfter: WaitTime},nil
	}
	// 判断 pvc 是否删除完毕,如果删除失败或者刚刚创建，等待5秒
	if ok:= pvcCtrl.ShrinkPVC(ctx, statefulPod, index-1); !ok {
		return ctrl.Result{RequeueAfter: WaitTime},nil
	}
	statefulPod.Status.PodStatusMes = statefulPod.Status.PodStatusMes[:index-1]
	statefulPod.Status.PVCStatusMes = statefulPod.Status.PVCStatusMes[:index-1]
	if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil{ // 更新失败，等待5秒
		return ctrl.Result{
			RequeueAfter: WaitTime,
		},nil
	}
	return ctrl.Result{},nil
}

// 维护 pod 状态
func (s *StatefulPodCtrl) maintain(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) (ctrl.Result,error) {
	podCtrl := podctrl.NewPodCtrl(s.Client)
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	// 检查pod是否有没有意外退出的，若有，则将其在statefulPod status的索引位置置为deleting ,若pod存在，状态为running，而statefulPod中记录的不是也返回索引值
	if index := podCtrl.PodIsOk(ctx, statefulPod); index != nil {
		if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil{
			return ctrl.Result{RequeueAfter: WaitTime},nil
		}
	}
	if index := podCtrl.MaintainPod(ctx, statefulPod); index != nil {
		return s.expansion(ctx, statefulPod, *index)
	}
	return ctrl.Result{},nil
}

// 设置 statefulPod finalizer
func (s *StatefulPodCtrl) setFinalizer(ctx context.Context, statefulPod *iapetosapiv1.StatefulPod) (ctrl.Result,error) {
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	myFinalizerName := iapetosapiv1.GroupVersion.String()
	if statefulPod.ObjectMeta.DeletionTimestamp.IsZero() {
		if !tools.MatchStringFromArray(statefulPod.Finalizers, myFinalizerName) { // finalizer未设置，则添加finalizer
			statefulPod.Finalizers = append(statefulPod.Finalizers, myFinalizerName)
			if _,err:=statefulPodHandler.Update(ctx, statefulPod);err!=nil {
				return ctrl.Result{RequeueAfter: WaitTime},err
			}
		}
	}
	return ctrl.Result{},nil
}

// 处理 pod 不同的 status
// pod 异常退出，重新拉起 pod
// node 节点失联，新建 pod、pvc
// pod running 状态，修改 statefulPod.status.PodStatusMes
func (s *StatefulPodCtrl) MonitorPodStatus(ctx context.Context, pod *corev1.Pod) (ctrl.Result,error) {
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	 obj,ok := statefulPodHandler.IsExists(ctx,types.NamespacedName{
		Namespace: pod.Namespace,
		Name:      pod.Annotations[ParentNmae],
	})
	if !ok {
		return ctrl.Result{},nil
	}
	statefulPod:=obj.(*iapetosapiv1.StatefulPod)
	index := tools.StringToInt(pod.Annotations["index"])
	podctl := podctrl.NewPodCtrl(s.Client)
	if ok := podctl.MonitorPodStatus(ctx, statefulPod, pod, &index); ok {
		if _,err:= statefulPodHandler.Update(ctx, statefulPod);err!=nil{
			return ctrl.Result{RequeueAfter: WaitTime}, nil
		}
	}
	return ctrl.Result{},nil
}

// 处理 pvc 不同的 status
func (s *StatefulPodCtrl) MonitorPVCStatus(ctx context.Context, pvc *corev1.PersistentVolumeClaim) (ctrl.Result,error) {
	statefulPodHandler:=statefulpod.NewStatefulPod(s.Client)
	obj,ok := statefulPodHandler.IsExists(ctx,types.NamespacedName{
		Namespace: pvc.Namespace,
		Name:      pvc.Annotations[ParentNmae],
	})
	if !ok {
		return ctrl.Result{},nil
	}
	statefulPod:=obj.(*iapetosapiv1.StatefulPod)
	index := tools.StringToInt(pvc.Annotations["index"])
	pvcCtrl := pvcctrl.NewPVCCtrl(s.Client)
	if ok := pvcCtrl.MonitorPVCStatus(ctx, statefulPod, pvc, index); ok {
		if _,err:= statefulPodHandler.Update(ctx, statefulPod);err!=nil{
			return ctrl.Result{RequeueAfter: WaitTime}, nil
		}
	}
	return ctrl.Result{},nil
}

